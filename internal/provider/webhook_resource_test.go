// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestWebhookAttributesFromModel_Payload covers the fully-populated case:
// basic auth, custom payload and an event with filters (mirrors a real
// production webhook payload).
func TestWebhookAttributesFromModel_Payload(t *testing.T) {
	ctx := context.Background()

	customPayload := `{"message": "{{event_type}} on {{entity_type}}"}`

	model := WebhookResourceModel{
		Name:                 types.StringValue("Publish/Unpublish records webhook"),
		URL:                  types.StringValue("https://events.example.com/datocms/records"),
		Headers:              types.MapValueMust(types.StringType, map[string]attr.Value{}),
		CustomPayload:        types.StringValue(customPayload),
		HTTPBasicUser:        types.StringValue("basic-user"),
		HTTPBasicPassword:    types.StringValue("basic-password"),
		Enabled:              types.BoolValue(true),
		PayloadAPIVersion:    types.StringValue("3"),
		NestedItemsInPayload: types.BoolValue(false),
		AutoRetry:            types.BoolValue(false),
		Events: types.ListValueMust(webhookEventObjectType, []attr.Value{
			types.ObjectValueMust(webhookEventObjectType.AttrTypes, map[string]attr.Value{
				"entity_type": types.StringValue("item"),
				"event_types": types.ListValueMust(types.StringType, []attr.Value{
					types.StringValue("publish"),
					types.StringValue("unpublish"),
				}),
				"filters": types.ListValueMust(webhookEventFilterObjectType, []attr.Value{
					types.ObjectValueMust(webhookEventFilterObjectType.AttrTypes, map[string]attr.Value{
						"entity_type": types.StringValue("item_type"),
						"entity_ids": types.ListValueMust(types.StringType, []attr.Value{
							types.StringValue("article"),
							types.StringValue("author"),
							types.StringValue("category"),
						}),
					}),
				}),
			}),
		}),
	}

	attrs, diags := webhookAttributesFromModel(ctx, &model)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	body := webhookPayload{Data: webhookData{
		Type:       "webhook",
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
	if data["type"] != "webhook" {
		t.Errorf("data.type = %v, want webhook", data["type"])
	}
	if _, hasID := data["id"]; hasID {
		t.Errorf("create payload must not include data.id, got %v", data["id"])
	}

	a := mustAssert[map[string]any](t, data["attributes"])
	if a["name"] != "Publish/Unpublish records webhook" {
		t.Errorf("name = %v", a["name"])
	}
	if a["url"] != "https://events.example.com/datocms/records" {
		t.Errorf("url = %v", a["url"])
	}
	if a["http_basic_user"] != "basic-user" || a["http_basic_password"] != "basic-password" {
		t.Errorf("basic auth = %v / %v", a["http_basic_user"], a["http_basic_password"])
	}
	if a["custom_payload"] != customPayload {
		t.Errorf("custom_payload = %v", a["custom_payload"])
	}
	if a["enabled"] != true || a["payload_api_version"] != "3" ||
		a["nested_items_in_payload"] != false || a["auto_retry"] != false {
		t.Errorf("unexpected flags: enabled=%v payload_api_version=%v nested=%v auto_retry=%v",
			a["enabled"], a["payload_api_version"], a["nested_items_in_payload"], a["auto_retry"])
	}

	// headers must serialize as an object (create requires it), not null.
	headers, ok := a["headers"].(map[string]any)
	if !ok {
		t.Fatalf("headers = %v, want empty object", a["headers"])
	}
	if len(headers) != 0 {
		t.Errorf("headers length = %d, want 0", len(headers))
	}

	events := mustAssert[[]any](t, a["events"])
	if len(events) != 1 {
		t.Fatalf("events length = %d, want 1", len(events))
	}
	event := mustAssert[map[string]any](t, events[0])
	if event["entity_type"] != "item" {
		t.Errorf("events[0].entity_type = %v, want item", event["entity_type"])
	}
	eventTypes := mustAssert[[]any](t, event["event_types"])
	if len(eventTypes) != 2 || eventTypes[0] != "publish" || eventTypes[1] != "unpublish" {
		t.Errorf("events[0].event_types = %v", eventTypes)
	}
	filters := mustAssert[[]any](t, event["filters"])
	if len(filters) != 1 {
		t.Fatalf("filters length = %d, want 1", len(filters))
	}
	filter := mustAssert[map[string]any](t, filters[0])
	if filter["entity_type"] != "item_type" {
		t.Errorf("filters[0].entity_type = %v, want item_type", filter["entity_type"])
	}
	ids := mustAssert[[]any](t, filter["entity_ids"])
	if len(ids) != 3 || ids[0] != "article" || ids[1] != "author" || ids[2] != "category" {
		t.Errorf("filters[0].entity_ids = %v", ids)
	}
}

// TestWebhookAttributesFromModel_NullableFields covers the minimal case:
// custom_payload and the basic auth pair omitted (they must serialize as
// explicit nulls, since create requires them even when null) and an event
// without filters (filters must be omitted from the entry).
func TestWebhookAttributesFromModel_NullableFields(t *testing.T) {
	ctx := context.Background()

	model := WebhookResourceModel{
		Name:                 types.StringValue("minimal"),
		URL:                  types.StringValue("https://example.com/hook"),
		Headers:              types.MapValueMust(types.StringType, map[string]attr.Value{}),
		CustomPayload:        types.StringNull(),
		HTTPBasicUser:        types.StringNull(),
		HTTPBasicPassword:    types.StringNull(),
		Enabled:              types.BoolValue(true),
		PayloadAPIVersion:    types.StringValue("3"),
		NestedItemsInPayload: types.BoolValue(false),
		AutoRetry:            types.BoolValue(false),
		Events: types.ListValueMust(webhookEventObjectType, []attr.Value{
			types.ObjectValueMust(webhookEventObjectType.AttrTypes, map[string]attr.Value{
				"entity_type": types.StringValue("build_trigger"),
				"event_types": types.ListValueMust(types.StringType, []attr.Value{
					types.StringValue("deploy_failed"),
				}),
				"filters": types.ListNull(webhookEventFilterObjectType),
			}),
		}),
	}

	attrs, diags := webhookAttributesFromModel(ctx, &model)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	raw, err := json.Marshal(webhookPayload{Data: webhookData{Type: "webhook", Attributes: attrs}})
	if err != nil {
		t.Fatalf("marshaling payload: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshaling payload: %v", err)
	}
	data := mustAssert[map[string]any](t, got["data"])
	a := mustAssert[map[string]any](t, data["attributes"])

	// Create-required nullable fields must be present as explicit nulls.
	for _, field := range []string{"custom_payload", "http_basic_user", "http_basic_password"} {
		v, present := a[field]
		if !present {
			t.Errorf("%s must be present in the payload (explicit null)", field)
			continue
		}
		if v != nil {
			t.Errorf("%s = %v, want null", field, v)
		}
	}

	events := mustAssert[[]any](t, a["events"])
	if len(events) != 1 {
		t.Fatalf("events length = %d, want 1", len(events))
	}
	event := mustAssert[map[string]any](t, events[0])
	if event["entity_type"] != "build_trigger" {
		t.Errorf("events[0].entity_type = %v, want build_trigger", event["entity_type"])
	}
	if v, present := event["filters"]; present {
		t.Errorf("expected filters to be omitted, got %v", v)
	}
}

// realWebhook21884 mirrors the shape of GET /webhooks/21884 (a production
// webhook without basic auth or custom payload; endpoint sanitized).
const realWebhook21884 = `{
	"url": "https://hooks.example.com/schedule",
	"name": "Programar Publicacion",
	"http_basic_user": null,
	"custom_payload": null,
	"http_basic_password": null,
	"headers": {"Accept": "application/json", "Content-Type": "application/json"},
	"events": [{"filters": [{"entity_ids": ["promotion", "exchange_campaign", "company"], "entity_type": "item_type"}], "entity_type": "item", "event_types": ["publish", "unpublish"]}],
	"enabled": true,
	"payload_api_version": "3",
	"nested_items_in_payload": false,
	"auto_retry": false
}`

// realWebhook31433 mirrors GET /webhooks/31433: same structure but with basic
// auth set (dummy secrets here), a Mustache custom payload and empty headers
// ({}, which is distinct from a missing object).
const realWebhook31433 = `{
	"url": "https://events.example.com/datocms/records",
	"name": "Notificar Publicacion",
	"http_basic_user": "dummy-user",
	"custom_payload": "{\"message\": \"{{event_type}} on {{entity_type}}\", \"entity_id\": \"{{#entity}}{{id}}{{/entity}}\"}",
	"http_basic_password": "dummy-password",
	"headers": {},
	"events": [{"filters": [{"entity_ids": ["promotion"], "entity_type": "item_type"}], "entity_type": "item", "event_types": ["publish", "unpublish"]}],
	"enabled": true,
	"payload_api_version": "3",
	"nested_items_in_payload": false,
	"auto_retry": false
}`

// TestWebhookRoundTrip_RealExamples maps two real production webhooks from
// their GET response into the model and back into a request payload, which
// must reproduce the original attributes exactly: explicit nulls for the
// nullable trio, headers {} kept as {} (never null), events filters intact
// and the Mustache custom_payload untouched.
func TestWebhookRoundTrip_RealExamples(t *testing.T) {
	ctx := context.Background()

	for name, raw := range map[string]string{
		"21884 no auth, null payload, headers set":  realWebhook21884,
		"31433 basic auth, mustache, empty headers": realWebhook31433,
	} {
		t.Run(name, func(t *testing.T) {
			var attrs webhookAttributes
			if err := json.Unmarshal([]byte(raw), &attrs); err != nil {
				t.Fatalf("unmarshaling fixture: %v", err)
			}
			webhook := webhookData{ID: "21884", Type: "webhook", Attributes: attrs}

			var model WebhookResourceModel
			if diags := modelFromWebhook(ctx, &webhook, &model); diags.HasError() {
				t.Fatalf("modelFromWebhook diagnostics: %v", diags)
			}

			roundTripped, diags := webhookAttributesFromModel(ctx, &model)
			if diags.HasError() {
				t.Fatalf("webhookAttributesFromModel diagnostics: %v", diags)
			}

			gotRaw, err := json.Marshal(roundTripped)
			if err != nil {
				t.Fatalf("marshaling round-tripped attributes: %v", err)
			}
			var got, want map[string]any
			if err := json.Unmarshal(gotRaw, &got); err != nil {
				t.Fatalf("unmarshaling round-tripped attributes: %v", err)
			}
			if err := json.Unmarshal([]byte(raw), &want); err != nil {
				t.Fatalf("unmarshaling fixture: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("round trip mismatch:\n got: %s\nwant: %s", gotRaw, raw)
			}

			// headers must survive as an object, never collapse to null.
			headers := mustAssert[map[string]any](t, got["headers"])
			wantHeaders := mustAssert[map[string]any](t, want["headers"])
			if len(headers) != len(wantHeaders) {
				t.Errorf("headers length = %d, want %d", len(headers), len(wantHeaders))
			}
		})
	}
}

// TestModelFromWebhook_PasswordPreservedOnNull ensures the defensive
// fallback: when a response carries a null http_basic_password, the value
// already in state is preserved instead of being cleared.
func TestModelFromWebhook_PasswordPreservedOnNull(t *testing.T) {
	ctx := context.Background()

	var attrs webhookAttributes
	if err := json.Unmarshal([]byte(realWebhook21884), &attrs); err != nil {
		t.Fatalf("unmarshaling fixture: %v", err)
	}
	webhook := webhookData{ID: "21884", Type: "webhook", Attributes: attrs}

	model := WebhookResourceModel{HTTPBasicPassword: types.StringValue("kept-secret")}
	if diags := modelFromWebhook(ctx, &webhook, &model); diags.HasError() {
		t.Fatalf("modelFromWebhook diagnostics: %v", diags)
	}
	if model.HTTPBasicPassword.ValueString() != "kept-secret" {
		t.Errorf("http_basic_password = %v, want kept-secret preserved", model.HTTPBasicPassword)
	}
	// The user is not preserved: the API returns it, so null means null.
	if !model.HTTPBasicUser.IsNull() {
		t.Errorf("http_basic_user = %v, want null", model.HTTPBasicUser)
	}
}

// newWebhookTestServer returns an httptest server implementing a minimal
// /webhooks CRUD backed by an in-memory map, recording the bearer token of
// every request.
func newWebhookTestServer(t *testing.T, seenTokens *[]string) (*httptest.Server, map[string]webhookData) {
	t.Helper()
	store := map[string]webhookData{}
	nextID := 21883

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Api-Version"); got != "3" {
			t.Errorf("X-Api-Version = %q, want 3", got)
		}
		*seenTokens = append(*seenTokens, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))

		id := strings.TrimPrefix(r.URL.Path, "/webhooks/")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/webhooks":
			var payload webhookPayload
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decoding create body: %v", err)
			}
			nextID++
			payload.Data.ID = strconv.Itoa(nextID)
			store[payload.Data.ID] = payload.Data
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(webhookPayload{Data: payload.Data})
		case r.Method == http.MethodGet && id != "":
			data, ok := store[id]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(webhookPayload{Data: data})
		case r.Method == http.MethodPut && id != "":
			if _, ok := store[id]; !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			var payload webhookPayload
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Errorf("decoding update body: %v", err)
			}
			payload.Data.ID = id
			store[id] = payload.Data
			_ = json.NewEncoder(w).Encode(webhookPayload{Data: payload.Data})
		case r.Method == http.MethodDelete && id != "":
			data, ok := store[id]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			delete(store, id)
			_ = json.NewEncoder(w).Encode(webhookPayload{Data: data})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)
	return server, store
}

// TestWebhookClientCRUD exercises the full client lifecycle against a mock
// API server using the real 31433-style payload: create, get (secrets echoed
// back), update, delete and the not-found mapping.
func TestWebhookClientCRUD(t *testing.T) {
	ctx := context.Background()
	var seenTokens []string
	server, store := newWebhookTestServer(t, &seenTokens)

	client := &DatoCMSClient{
		APIToken:   "default-token",
		BaseURL:    server.URL,
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}

	var attrs webhookAttributes
	if err := json.Unmarshal([]byte(realWebhook31433), &attrs); err != nil {
		t.Fatalf("unmarshaling fixture: %v", err)
	}

	created, err := client.CreateWebhook(ctx, attrs)
	if err != nil {
		t.Fatalf("CreateWebhook: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("CreateWebhook returned empty ID")
	}

	got, err := client.GetWebhook(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetWebhook: %v", err)
	}
	if got.Attributes.HTTPBasicUser == nil || *got.Attributes.HTTPBasicUser != "dummy-user" ||
		got.Attributes.HTTPBasicPassword == nil || *got.Attributes.HTTPBasicPassword != "dummy-password" {
		t.Errorf("basic auth not echoed back: %+v", got.Attributes)
	}
	if got.Attributes.Headers == nil || len(got.Attributes.Headers) != 0 {
		t.Errorf("headers = %v, want empty object", got.Attributes.Headers)
	}
	if len(got.Attributes.Events) != 1 || len(got.Attributes.Events[0].Filters) != 1 {
		t.Errorf("events/filters not preserved: %+v", got.Attributes.Events)
	}

	attrs.Name = "Notificar Publicacion v2"
	attrs.Enabled = false
	updated, err := client.UpdateWebhook(ctx, created.ID, attrs)
	if err != nil {
		t.Fatalf("UpdateWebhook: %v", err)
	}
	if updated.Attributes.Name != "Notificar Publicacion v2" || updated.Attributes.Enabled {
		t.Errorf("unexpected updated webhook: %+v", updated.Attributes)
	}

	if err := client.DeleteWebhook(ctx, created.ID); err != nil {
		t.Fatalf("DeleteWebhook: %v", err)
	}
	if len(store) != 0 {
		t.Errorf("store length = %d after delete, want 0", len(store))
	}
	if _, err := client.GetWebhook(ctx, created.ID); !errors.Is(err, errNotFound) {
		t.Errorf("GetWebhook after delete = %v, want errNotFound", err)
	}
}

// TestWebhookClient_ProjectTokenSelection covers the multi-project case: two
// projects declared in api_tokens, one webhook created per project, each
// request carrying the matching token, and unknown projects failing before
// any request is made.
func TestWebhookClient_ProjectTokenSelection(t *testing.T) {
	ctx := context.Background()
	var seenTokens []string
	server, _ := newWebhookTestServer(t, &seenTokens)

	base := &DatoCMSClient{
		APIToken: "default-token",
		APITokens: map[string]string{
			"store-one": "token-one",
			"store-two": "token-two",
		},
		BaseURL:    server.URL,
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}

	var attrs webhookAttributes
	if err := json.Unmarshal([]byte(realWebhook21884), &attrs); err != nil {
		t.Fatalf("unmarshaling fixture: %v", err)
	}

	for _, project := range []string{"store-one", "store-two"} {
		scoped, err := base.forProject(project)
		if err != nil {
			t.Fatalf("forProject(%q): %v", project, err)
		}
		if _, err := scoped.CreateWebhook(ctx, attrs); err != nil {
			t.Fatalf("CreateWebhook for %q: %v", project, err)
		}
	}

	want := []string{"token-one", "token-two"}
	if !reflect.DeepEqual(seenTokens, want) {
		t.Errorf("tokens seen = %v, want %v", seenTokens, want)
	}

	if _, err := base.forProject("store-three"); err == nil {
		t.Fatalf("forProject(store-three) = nil, want error")
	}
	if len(seenTokens) != len(want) {
		t.Errorf("unknown project still reached the server: %v", seenTokens)
	}
}
