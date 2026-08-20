// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestAccessTokenAttributesFromModel_WithRole covers the fully-populated
// case: all surface flags set and a role attached (the relationships object
// must carry a role linkage).
func TestAccessTokenAttributesFromModel_WithRole(t *testing.T) {
	model := AccessTokenResourceModel{
		Name:                types.StringValue("Terraform"),
		RoleID:              types.StringValue("475455"),
		CanAccessCda:        types.BoolValue(true),
		CanAccessCdaPreview: types.BoolValue(true),
		CanAccessCma:        types.BoolValue(true),
	}

	attrs, roleID := accessTokenAttributesFromModel(&model)

	body := accessTokenPayload{Data: accessTokenData{
		Type:          "access_token",
		Attributes:    attrs,
		Relationships: newAccessTokenRelationships(roleID),
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
	if data["type"] != "access_token" {
		t.Errorf("data.type = %v, want access_token", data["type"])
	}
	if _, hasID := data["id"]; hasID {
		t.Errorf("create payload must not include data.id, got %v", data["id"])
	}

	a := mustAssert[map[string]any](t, data["attributes"])
	if a["name"] != "Terraform" {
		t.Errorf("name = %v", a["name"])
	}
	if a["can_access_cda"] != true || a["can_access_cda_preview"] != true || a["can_access_cma"] != true {
		t.Errorf("unexpected flags: cda=%v cda_preview=%v cma=%v",
			a["can_access_cda"], a["can_access_cda_preview"], a["can_access_cma"])
	}
	// The create schema accepts exactly name plus the three flags
	// (additionalProperties: false): the response-only token attribute must
	// never be serialized.
	if v, present := a["token"]; present {
		t.Errorf("token must not be present in the payload, got %v", v)
	}
	if len(a) != 4 {
		t.Errorf("attributes length = %d, want 4 (%v)", len(a), a)
	}

	rels := mustAssert[map[string]any](t, data["relationships"])
	role := mustAssert[map[string]any](t, rels["role"])
	linkage, ok := role["data"].(map[string]any)
	if !ok {
		t.Fatalf("relationships.role.data = %v, want linkage object", role["data"])
	}
	if linkage["type"] != "role" || linkage["id"] != "475455" {
		t.Errorf("role linkage = %v, want {type: role, id: 475455}", linkage)
	}
}

// TestAccessTokenAttributesFromModel_WithoutRole covers the minimal case:
// role_id omitted (relationships.role.data must serialize as an explicit
// null, since create requires the relationships object) and all flags false.
func TestAccessTokenAttributesFromModel_WithoutRole(t *testing.T) {
	model := AccessTokenResourceModel{
		Name:                types.StringValue("CDA read-only token"),
		RoleID:              types.StringNull(),
		CanAccessCda:        types.BoolValue(true),
		CanAccessCdaPreview: types.BoolValue(false),
		CanAccessCma:        types.BoolValue(false),
	}

	attrs, roleID := accessTokenAttributesFromModel(&model)
	if roleID != nil {
		t.Fatalf("roleID = %v, want nil", *roleID)
	}

	raw, err := json.Marshal(accessTokenPayload{Data: accessTokenData{
		Type:          "access_token",
		Attributes:    attrs,
		Relationships: newAccessTokenRelationships(roleID),
	}})
	if err != nil {
		t.Fatalf("marshaling payload: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshaling payload: %v", err)
	}

	data := mustAssert[map[string]any](t, got["data"])
	a := mustAssert[map[string]any](t, data["attributes"])
	if a["can_access_cda"] != true || a["can_access_cda_preview"] != false || a["can_access_cma"] != false {
		t.Errorf("unexpected flags: cda=%v cda_preview=%v cma=%v",
			a["can_access_cda"], a["can_access_cda_preview"], a["can_access_cma"])
	}

	rels := mustAssert[map[string]any](t, data["relationships"])
	role, ok := rels["role"].(map[string]any)
	if !ok {
		t.Fatalf("relationships.role = %v, want object", rels["role"])
	}
	v, present := role["data"]
	if !present {
		t.Fatalf("relationships.role.data must be present (explicit null)")
	}
	if v != nil {
		t.Errorf("relationships.role.data = %v, want null", v)
	}
}

// TestModelFromAccessToken_TokenPreservation checks the secret drift
// protection: a null attributes.token in the response (API masks it for
// callers without can_manage_access_tokens) must preserve the value already
// stored in the state.
func TestModelFromAccessToken_TokenPreservation(t *testing.T) {
	data := AccessTokenResourceModel{
		ID:    types.StringValue("123456"),
		Token: types.StringValue("stored-secret"),
	}

	api := &accessTokenData{
		ID:   "123456",
		Type: "access_token",
		Attributes: accessTokenAttributes{
			Name:                "Terraform",
			CanAccessCda:        true,
			CanAccessCdaPreview: true,
			CanAccessCma:        true,
			Token:               nil, // masked by the API
		},
	}

	modelFromAccessToken(api, &data)

	if data.Token.ValueString() != "stored-secret" {
		t.Errorf("token = %v, want stored-secret preserved", data.Token)
	}
	if !data.RoleID.IsNull() {
		t.Errorf("role_id = %v, want null (no relationships in response)", data.RoleID)
	}

	// A non-null token in the response replaces the stored one.
	secret := "new-secret"
	api.Attributes.Token = &secret
	api.Relationships = newAccessTokenRelationships(stringPtr(types.StringValue("475455")))

	modelFromAccessToken(api, &data)

	if data.Token.ValueString() != "new-secret" {
		t.Errorf("token = %v, want new-secret", data.Token)
	}
	if data.RoleID.ValueString() != "475455" {
		t.Errorf("role_id = %v, want 475455", data.RoleID)
	}
}
