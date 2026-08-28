// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func workflowStageValue(id, name string, description types.String, initial bool) attr.Value {
	return types.ObjectValueMust(workflowStageObjectType.AttrTypes, map[string]attr.Value{
		"id":          types.StringValue(id),
		"name":        types.StringValue(name),
		"description": description,
		"initial":     types.BoolValue(initial),
	})
}

// TestWorkflowAttributesFromModel_Payload covers the fully-populated case,
// mirroring a real production workflow: an initial stage with a description
// plus a stage without one (description must be omitted, not sent as null).
func TestWorkflowAttributesFromModel_Payload(t *testing.T) {
	ctx := context.Background()

	model := WorkflowResourceModel{
		Name:   types.StringValue("Approval by publisher"),
		APIKey: types.StringValue("approval_by_publisher"),
		Stages: types.ListValueMust(workflowStageObjectType, []attr.Value{
			workflowStageValue("work_in_progress", "Work in progress",
				types.StringValue("Content is being written"), true),
			workflowStageValue("in_review", "In review", types.StringNull(), false),
		}),
	}

	attrs, diags := workflowAttributesFromModel(ctx, &model)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	body := workflowPayload{Data: workflowData{
		Type:       "workflow",
		Attributes: attrs,
	}}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshaling payload: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshaling payload: %v", err)
	}

	data := mustAssert[map[string]any](t, got["data"])
	if data["type"] != "workflow" {
		t.Errorf("data.type = %v, want workflow", data["type"])
	}
	if _, hasID := data["id"]; hasID {
		t.Errorf("create payload must not include data.id, got %v", data["id"])
	}

	a := mustAssert[map[string]any](t, data["attributes"])
	if a["name"] != "Approval by publisher" {
		t.Errorf("name = %v", a["name"])
	}
	if a["api_key"] != "approval_by_publisher" {
		t.Errorf("api_key = %v", a["api_key"])
	}

	stages := mustAssert[[]any](t, a["stages"])
	if len(stages) != 2 {
		t.Fatalf("stages length = %d, want 2", len(stages))
	}

	first := mustAssert[map[string]any](t, stages[0])
	if first["id"] != "work_in_progress" || first["name"] != "Work in progress" {
		t.Errorf("stages[0] = %v", first)
	}
	if first["description"] != "Content is being written" {
		t.Errorf("stages[0].description = %v", first["description"])
	}
	if first["initial"] != true {
		t.Errorf("stages[0].initial = %v, want true", first["initial"])
	}

	second := mustAssert[map[string]any](t, stages[1])
	if second["id"] != "in_review" || second["name"] != "In review" {
		t.Errorf("stages[1] = %v", second)
	}
	// A null description must be omitted from the payload (optional field).
	if v, present := second["description"]; present {
		t.Errorf("expected stages[1].description to be omitted, got %v", v)
	}
	// initial must always serialize, even when false.
	if v, present := second["initial"]; !present || v != false {
		t.Errorf("stages[1].initial = %v (present=%v), want explicit false", v, present)
	}
}

// TestModelFromWorkflow covers the response mapping: a stage without
// description or initial maps to a null description and initial = false.
func TestModelFromWorkflow(t *testing.T) {
	ctx := context.Background()

	raw := `{
		"id": "949",
		"type": "workflow",
		"attributes": {
			"name": "Approval by publisher",
			"api_key": "approval_by_publisher",
			"stages": [
				{"id": "work_in_progress", "name": "Work in progress", "description": "Content is being written", "initial": true},
				{"id": "in_review", "name": "In review"}
			]
		}
	}`
	var workflow workflowData
	if err := json.Unmarshal([]byte(raw), &workflow); err != nil {
		t.Fatalf("unmarshaling fixture: %v", err)
	}

	var model WorkflowResourceModel
	diags := modelFromWorkflow(ctx, &workflow, &model)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if model.ID.ValueString() != "949" {
		t.Errorf("id = %v, want 949", model.ID)
	}
	if model.Name.ValueString() != "Approval by publisher" {
		t.Errorf("name = %v", model.Name)
	}
	if model.APIKey.ValueString() != "approval_by_publisher" {
		t.Errorf("api_key = %v", model.APIKey)
	}

	var stages []workflowStageModel
	diags = model.Stages.ElementsAs(ctx, &stages, false)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(stages) != 2 {
		t.Fatalf("stages length = %d, want 2", len(stages))
	}
	if stages[0].ID.ValueString() != "work_in_progress" ||
		stages[0].Description.ValueString() != "Content is being written" ||
		!stages[0].Initial.ValueBool() {
		t.Errorf("unexpected stages[0]: %+v", stages[0])
	}
	if stages[1].ID.ValueString() != "in_review" || !stages[1].Description.IsNull() ||
		stages[1].Initial.ValueBool() {
		t.Errorf("unexpected stages[1]: %+v", stages[1])
	}
}

// newWorkflowTestServer returns an httptest server implementing a minimal
// /workflows CRUD backed by an in-memory map, and the store itself. Every
// request is checked for the expected auth and version headers.
func newWorkflowTestServer(t *testing.T, wantToken string) (*httptest.Server, map[string]workflowData) {
	t.Helper()
	store := map[string]workflowData{}
	nextID := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+wantToken {
			t.Errorf("Authorization = %q, want Bearer %s", got, wantToken)
		}
		if got := r.Header.Get("X-Api-Version"); got != "3" {
			t.Errorf("X-Api-Version = %q, want 3", got)
		}

		id := strings.TrimPrefix(r.URL.Path, "/workflows/")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/workflows":
			var payload workflowPayload
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decoding create body: %v", err)
			}
			nextID++
			payload.Data.ID = "94" + strconv.Itoa(nextID)
			store[payload.Data.ID] = payload.Data
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(workflowPayload{Data: payload.Data})

		case r.Method == http.MethodGet && id != "":
			data, ok := store[id]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"data":[{"id":"NOT_FOUND"}]}`))
				return
			}
			_ = json.NewEncoder(w).Encode(workflowPayload{Data: data})

		case r.Method == http.MethodPut && id != "":
			if _, ok := store[id]; !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			var payload workflowPayload
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decoding update body: %v", err)
			}
			payload.Data.ID = id
			store[id] = payload.Data
			_ = json.NewEncoder(w).Encode(workflowPayload{Data: payload.Data})

		case r.Method == http.MethodDelete && id != "":
			data, ok := store[id]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			delete(store, id)
			_ = json.NewEncoder(w).Encode(workflowPayload{Data: data})

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)
	return server, store
}

func testWorkflowAttrs() workflowAttributes {
	description := "Editor is waiting for approval"
	//nolint:gosec // G101 false positive: api_key is the workflow slug, not a credential
	return workflowAttributes{
		Name:   "Approval by publisher",
		APIKey: "approval_by_publisher",
		Stages: []workflowStage{
			{ID: "work_in_progress", Name: "Work in progress", Initial: true},
			{ID: "in_review", Name: "In review", Description: &description},
		},
	}
}

// TestWorkflowClientCRUD exercises the full client lifecycle against a mock
// API server: create, get, update, delete and the not-found mapping.
func TestWorkflowClientCRUD(t *testing.T) {
	ctx := context.Background()
	server, store := newWorkflowTestServer(t, "default-token")

	client := &DatoCMSClient{
		APIToken:   "default-token",
		BaseURL:    server.URL,
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}

	created, err := client.CreateWorkflow(ctx, testWorkflowAttrs())
	if err != nil {
		t.Fatalf("CreateWorkflow: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("CreateWorkflow returned empty ID")
	}
	if created.Attributes.Name != "Approval by publisher" || len(created.Attributes.Stages) != 2 {
		t.Errorf("unexpected created workflow: %+v", created)
	}

	got, err := client.GetWorkflow(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetWorkflow: %v", err)
	}
	if got.Attributes.APIKey != "approval_by_publisher" {
		t.Errorf("api_key = %v", got.Attributes.APIKey)
	}
	if got.Attributes.Stages[1].Description == nil ||
		*got.Attributes.Stages[1].Description != "Editor is waiting for approval" {
		t.Errorf("stages[1].description = %v", got.Attributes.Stages[1].Description)
	}

	updatedAttrs := testWorkflowAttrs()
	updatedAttrs.Name = "Approval by editors"
	updatedAttrs.Stages = append(updatedAttrs.Stages, workflowStage{ID: "approved", Name: "Approved"})
	updated, err := client.UpdateWorkflow(ctx, created.ID, updatedAttrs)
	if err != nil {
		t.Fatalf("UpdateWorkflow: %v", err)
	}
	if updated.Attributes.Name != "Approval by editors" || len(updated.Attributes.Stages) != 3 {
		t.Errorf("unexpected updated workflow: %+v", updated)
	}
	if store[created.ID].Attributes.Name != "Approval by editors" {
		t.Errorf("server store not updated: %+v", store[created.ID])
	}

	if err := client.DeleteWorkflow(ctx, created.ID); err != nil {
		t.Fatalf("DeleteWorkflow: %v", err)
	}
	if len(store) != 0 {
		t.Errorf("store length = %d after delete, want 0", len(store))
	}

	// A read after deletion must map to errNotFound (resource gone).
	if _, err := client.GetWorkflow(ctx, created.ID); !errors.Is(err, errNotFound) {
		t.Errorf("GetWorkflow after delete = %v, want errNotFound", err)
	}
	if err := client.DeleteWorkflow(ctx, created.ID); !errors.Is(err, errNotFound) {
		t.Errorf("DeleteWorkflow after delete = %v, want errNotFound", err)
	}
}

// TestWorkflowClient_APIError covers non-404 API failures: the error must
// carry the status and body and must not be confused with errNotFound.
func TestWorkflowClient_APIError(t *testing.T) {
	ctx := context.Background()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"data":[{"id":"INVALID_FIELD","attributes":{"details":{"field":"api_key"}}}]}`))
	}))
	t.Cleanup(server.Close)

	client := &DatoCMSClient{
		APIToken:   "default-token",
		BaseURL:    server.URL,
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}

	_, err := client.CreateWorkflow(ctx, testWorkflowAttrs())
	if err == nil {
		t.Fatalf("CreateWorkflow = nil error, want 422 failure")
	}
	if errors.Is(err, errNotFound) {
		t.Errorf("422 must not map to errNotFound: %v", err)
	}
	for _, want := range []string{"422", "INVALID_FIELD"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err.Error(), want)
		}
	}
}

// TestWorkflowClient_ProjectTokenSelection ensures that resolving projects
// through the api_tokens map sends each project's own token, mirroring what
// clientForProject does for a resource with the project attribute set. Two
// projects are declared and one workflow is created per project; each request
// must carry the matching token.
func TestWorkflowClient_ProjectTokenSelection(t *testing.T) {
	ctx := context.Background()

	var seenTokens []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenTokens = append(seenTokens, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		var payload workflowPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decoding create body: %v", err)
		}
		payload.Data.ID = "949"
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(payload)
	}))
	t.Cleanup(server.Close)

	base := &DatoCMSClient{
		APIToken: "default-token",
		APITokens: map[string]string{
			"store-one": "token-one",
			"store-two": "token-two",
		},
		BaseURL:    server.URL,
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}

	for _, project := range []string{"store-one", "store-two"} {
		scoped, err := base.forProject(project)
		if err != nil {
			t.Fatalf("forProject(%q): %v", project, err)
		}
		if _, err := scoped.CreateWorkflow(ctx, testWorkflowAttrs()); err != nil {
			t.Fatalf("CreateWorkflow for %q: %v", project, err)
		}
	}

	want := []string{"token-one", "token-two"}
	if len(seenTokens) != len(want) {
		t.Fatalf("requests seen = %d, want %d", len(seenTokens), len(want))
	}
	for i, token := range want {
		if seenTokens[i] != token {
			t.Errorf("request %d used token %q, want %q", i, seenTokens[i], token)
		}
	}

	// An unknown project must fail before any request is made, naming the
	// project and the available keys without leaking token values.
	_, err := base.forProject("store-three")
	if err == nil {
		t.Fatalf("forProject(store-three) = nil, want error")
	}
	for _, wantMention := range []string{`"store-three"`, "store-one", "store-two"} {
		if !strings.Contains(err.Error(), wantMention) {
			t.Errorf("error %q does not mention %q", err.Error(), wantMention)
		}
	}
	for _, leaked := range []string{"token-one", "token-two", "default-token"} {
		if strings.Contains(err.Error(), leaked) {
			t.Errorf("error %q leaks token %q", err.Error(), leaked)
		}
	}
	if len(seenTokens) != len(want) {
		t.Errorf("unknown project still reached the server: %v", seenTokens)
	}
}

// TestWorkflowImportID covers the compound import ID form used by
// ImportState ("project/id" and plain "id").
func TestWorkflowImportID(t *testing.T) {
	project, id := parseImportID("store-one/949")
	if project != "store-one" || id != "949" {
		t.Errorf("parseImportID(store-one/949) = (%q, %q)", project, id)
	}
	project, id = parseImportID("949")
	if project != "" || id != "949" {
		t.Errorf("parseImportID(949) = (%q, %q)", project, id)
	}
}
