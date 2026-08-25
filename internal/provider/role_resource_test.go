// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func emptyRoleModelLists(m *RoleResourceModel) {
	m.PositiveItemTypePermissions = types.ListValueMust(itemTypePermissionObjectType, []attr.Value{})
	m.NegativeItemTypePermissions = types.ListValueMust(itemTypePermissionObjectType, []attr.Value{})
	m.PositiveUploadPermissions = types.ListValueMust(uploadPermissionObjectType, []attr.Value{})
	m.NegativeUploadPermissions = types.ListValueMust(uploadPermissionObjectType, []attr.Value{})
	m.PositiveBuildTriggerPermissions = types.ListValueMust(buildTriggerPermissionObjectType, []attr.Value{})
	m.NegativeBuildTriggerPermissions = types.ListValueMust(buildTriggerPermissionObjectType, []attr.Value{})
	m.PositiveSearchIndexPermissions = types.ListValueMust(searchIndexPermissionObjectType, []attr.Value{})
	m.NegativeSearchIndexPermissions = types.ListValueMust(searchIndexPermissionObjectType, []attr.Value{})
	m.InheritsPermissionsFrom = types.ListValueMust(types.StringType, []attr.Value{})
}

func TestRoleAttributesFromModel_Payload(t *testing.T) {
	ctx := context.Background()

	model := RoleResourceModel{
		Name:                          types.StringValue("developer"),
		CanEditSite:                   types.BoolValue(false),
		CanEditFavicon:                types.BoolValue(false),
		CanEditSchema:                 types.BoolValue(true),
		CanManageMenu:                 types.BoolValue(false),
		CanEditEnvironment:            types.BoolValue(false),
		CanPromoteEnvironments:        types.BoolValue(false),
		CanManageEnvironments:         types.BoolValue(false),
		CanManageUsers:                types.BoolValue(false),
		CanManageSharedFilters:        types.BoolValue(false),
		CanManageSearchIndexes:        types.BoolValue(false),
		CanManageUploadCollections:    types.BoolValue(false),
		CanManageBuildTriggers:        types.BoolValue(false),
		CanManageWebhooks:             types.BoolValue(false),
		CanManageSSO:                  types.BoolValue(false),
		CanManageWorkflows:            types.BoolValue(false),
		CanManageAccessTokens:         types.BoolValue(false),
		CanAccessAuditLog:             types.BoolValue(false),
		CanPerformSiteSearch:          types.BoolValue(false),
		CanAccessBuildEventsLog:       types.BoolValue(false),
		CanAccessSearchIndexEventsLog: types.BoolValue(true),
		EnvironmentsAccess:            types.StringValue("primary_only"),
	}
	emptyRoleModelLists(&model)

	model.PositiveItemTypePermissions = types.ListValueMust(itemTypePermissionObjectType, []attr.Value{
		types.ObjectValueMust(itemTypePermissionObjectType.AttrTypes, map[string]attr.Value{
			"action":             types.StringValue("all"),
			"environment":        types.StringValue("main"),
			"item_type":          types.StringNull(),
			"workflow":           types.StringValue("000002"),
			"on_stage":           types.StringNull(),
			"to_stage":           types.StringNull(),
			"on_creator":         types.StringValue("anyone"),
			"localization_scope": types.StringValue("all"),
			"locale":             types.StringNull(),
		}),
	})
	model.PositiveUploadPermissions = types.ListValueMust(uploadPermissionObjectType, []attr.Value{
		types.ObjectValueMust(uploadPermissionObjectType.AttrTypes, map[string]attr.Value{
			"action":                    types.StringValue("all"),
			"environment":               types.StringValue("main"),
			"upload_collection":         types.StringNull(),
			"move_to_upload_collection": types.StringNull(),
			"on_creator":                types.StringValue("anyone"),
			"localization_scope":        types.StringValue("all"),
			"locale":                    types.StringNull(),
		}),
	})
	model.PositiveBuildTriggerPermissions = types.ListValueMust(buildTriggerPermissionObjectType, []attr.Value{
		types.ObjectValueMust(buildTriggerPermissionObjectType.AttrTypes, map[string]attr.Value{
			"build_trigger": types.StringNull(),
		}),
	})
	model.PositiveSearchIndexPermissions = types.ListValueMust(searchIndexPermissionObjectType, []attr.Value{
		types.ObjectValueMust(searchIndexPermissionObjectType.AttrTypes, map[string]attr.Value{
			"search_index": types.StringValue("42"),
		}),
	})
	model.InheritsPermissionsFrom = types.ListValueMust(types.StringType, []attr.Value{
		types.StringValue("000001"),
	})

	attrs, inheritsFrom, diags := roleAttributesFromModel(ctx, &model)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	body := rolePayload{Data: roleData{
		Type:          "role",
		Attributes:    attrs,
		Relationships: newRoleRelationships(inheritsFrom),
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
	if data["type"] != "role" {
		t.Errorf("data.type = %v, want role", data["type"])
	}
	if _, hasID := data["id"]; hasID {
		t.Errorf("create payload must not include data.id, got %v", data["id"])
	}

	a := mustAssert[map[string]any](t, data["attributes"])
	if a["name"] != "developer" {
		t.Errorf("name = %v, want developer", a["name"])
	}
	if a["can_edit_schema"] != true || a["can_access_search_index_events_log"] != true {
		t.Errorf("expected can_edit_schema and can_access_search_index_events_log true, got %v / %v",
			a["can_edit_schema"], a["can_access_search_index_events_log"])
	}
	if a["can_edit_site"] != false {
		t.Errorf("can_edit_site = %v, want false", a["can_edit_site"])
	}
	if a["environments_access"] != "primary_only" {
		t.Errorf("environments_access = %v, want primary_only", a["environments_access"])
	}

	// All 20 can_* flags must be present in the payload.
	flags := []string{
		"can_edit_site", "can_edit_favicon", "can_edit_schema", "can_manage_menu",
		"can_edit_environment", "can_promote_environments", "can_manage_environments",
		"can_manage_users", "can_manage_shared_filters", "can_manage_search_indexes",
		"can_manage_upload_collections", "can_manage_build_triggers", "can_manage_webhooks",
		"can_manage_sso", "can_manage_workflows", "can_manage_access_tokens",
		"can_access_audit_log", "can_perform_site_search", "can_access_build_events_log",
		"can_access_search_index_events_log",
	}
	for _, flag := range flags {
		if _, ok := a[flag]; !ok {
			t.Errorf("missing flag %s in payload attributes", flag)
		}
	}

	perms := mustAssert[[]any](t, a["positive_item_type_permissions"])
	if len(perms) != 1 {
		t.Fatalf("positive_item_type_permissions length = %d, want 1", len(perms))
	}
	perm := mustAssert[map[string]any](t, perms[0])
	if perm["action"] != "all" || perm["environment"] != "main" || perm["workflow"] != "000002" ||
		perm["on_creator"] != "anyone" || perm["localization_scope"] != "all" {
		t.Errorf("unexpected item type permission: %v", perm)
	}
	// Null optional fields are omitted, never sent as empty strings.
	for _, field := range []string{"item_type", "on_stage", "to_stage", "locale"} {
		if v, ok := perm[field]; ok {
			t.Errorf("expected %s to be omitted, got %v", field, v)
		}
	}

	uploadPerms := mustAssert[[]any](t, a["positive_upload_permissions"])
	if len(uploadPerms) != 1 {
		t.Fatalf("positive_upload_permissions length = %d, want 1", len(uploadPerms))
	}
	uploadPerm := mustAssert[map[string]any](t, uploadPerms[0])
	if uploadPerm["action"] != "all" || uploadPerm["environment"] != "main" {
		t.Errorf("unexpected upload permission: %v", uploadPerm)
	}
	if v, ok := uploadPerm["upload_collection"]; ok {
		t.Errorf("expected upload_collection to be omitted, got %v", v)
	}

	// build_trigger: null must serialize explicitly (covers every trigger).
	btPerms := mustAssert[[]any](t, a["positive_build_trigger_permissions"])
	if len(btPerms) != 1 {
		t.Fatalf("positive_build_trigger_permissions length = %d, want 1", len(btPerms))
	}
	btPerm := mustAssert[map[string]any](t, btPerms[0])
	if bt, ok := btPerm["build_trigger"]; !ok || bt != nil {
		t.Errorf("build_trigger = %v (present=%v), want explicit null", bt, ok)
	}

	siPerms := mustAssert[[]any](t, a["positive_search_index_permissions"])
	if len(siPerms) != 1 {
		t.Fatalf("positive_search_index_permissions length = %d, want 1", len(siPerms))
	}
	siPerm := mustAssert[map[string]any](t, siPerms[0])
	if siPerm["search_index"] != "42" {
		t.Errorf("unexpected search index permission: %v", siPerms[0])
	}

	// Declared empty arrays must serialize as [], not null (wholesale
	// replacement / clear).
	for _, field := range []string{
		"negative_item_type_permissions", "negative_upload_permissions",
		"negative_build_trigger_permissions", "negative_search_index_permissions",
	} {
		v, ok := a[field]
		if !ok || v == nil {
			t.Errorf("%s must serialize as an empty array, got %v (present=%v)", field, v, ok)
			continue
		}
		if arr := mustAssert[[]any](t, v); len(arr) != 0 {
			t.Errorf("%s length = %d, want 0", field, len(arr))
		}
	}

	rels := mustAssert[map[string]any](t, data["relationships"])
	inherits := mustAssert[map[string]any](t, rels["inherits_permissions_from"])
	linkages := mustAssert[[]any](t, inherits["data"])
	if len(linkages) != 1 {
		t.Fatalf("inherits_permissions_from linkages = %d, want 1", len(linkages))
	}
	linkage := mustAssert[map[string]any](t, linkages[0])
	if linkage["type"] != "role" || linkage["id"] != "000001" {
		t.Errorf("unexpected linkage: %v", linkage)
	}
}

// TestRoleAttributesFromModel_OmittedContentListsAreNotSent covers the
// "preserve content permissions" semantics: item/upload permission lists that
// are null (omitted on update) or unknown (omitted on create) in the model
// must be absent from the JSON payload so the API leaves them unchanged.
func TestRoleAttributesFromModel_OmittedContentListsAreNotSent(t *testing.T) {
	ctx := context.Background()

	for name, listValue := range map[string]func() types.List{
		"null": func() types.List { return types.ListNull(itemTypePermissionObjectType) },
		"unknown": func() types.List {
			return types.ListUnknown(itemTypePermissionObjectType)
		},
	} {
		t.Run(name, func(t *testing.T) {
			model := RoleResourceModel{Name: types.StringValue("developer")}
			emptyRoleModelLists(&model)
			model.PositiveItemTypePermissions = listValue()
			if name == "null" {
				model.NegativeItemTypePermissions = types.ListNull(itemTypePermissionObjectType)
				model.PositiveUploadPermissions = types.ListNull(uploadPermissionObjectType)
				model.NegativeUploadPermissions = types.ListNull(uploadPermissionObjectType)
			} else {
				model.NegativeItemTypePermissions = types.ListUnknown(itemTypePermissionObjectType)
				model.PositiveUploadPermissions = types.ListUnknown(uploadPermissionObjectType)
				model.NegativeUploadPermissions = types.ListUnknown(uploadPermissionObjectType)
			}

			attrs, _, diags := roleAttributesFromModel(ctx, &model)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}

			raw, err := json.Marshal(attrs)
			if err != nil {
				t.Fatalf("marshaling attributes: %v", err)
			}
			var a map[string]any
			if err := json.Unmarshal(raw, &a); err != nil {
				t.Fatalf("unmarshaling attributes: %v", err)
			}

			for _, field := range []string{
				"positive_item_type_permissions", "negative_item_type_permissions",
				"positive_upload_permissions", "negative_upload_permissions",
			} {
				if v, ok := a[field]; ok {
					t.Errorf("expected %s to be omitted from the payload, got %v", field, v)
				}
			}
			// Build trigger and search index lists are first-screen managed
			// lists and must still be sent (as [] here).
			for _, field := range []string{
				"positive_build_trigger_permissions", "negative_build_trigger_permissions",
				"positive_search_index_permissions", "negative_search_index_permissions",
			} {
				v, ok := a[field]
				if !ok || v == nil {
					t.Errorf("expected %s to serialize as [], got %v (present=%v)", field, v, ok)
				}
			}
		})
	}
}

// TestCompleteContentPermissionPairs covers the API constraint that the
// positive and negative halves of each item/upload pair must be both
// present or both absent: a partially declared pair gets its missing half
// filled with [], while fully absent pairs stay absent.
func TestCompleteContentPermissionPairs(t *testing.T) {
	item := []roleItemTypePermission{{Action: "all"}}
	upload := []roleUploadPermission{{Action: "read"}}

	attrs := roleAttributes{
		PositiveItemTypePermissions: &item,
		NegativeUploadPermissions:   &upload,
	}
	completeContentPermissionPairs(&attrs)

	if attrs.NegativeItemTypePermissions == nil || len(*attrs.NegativeItemTypePermissions) != 0 {
		t.Errorf("negative item half not completed with []: %v", attrs.NegativeItemTypePermissions)
	}
	if attrs.PositiveUploadPermissions == nil || len(*attrs.PositiveUploadPermissions) != 0 {
		t.Errorf("positive upload half not completed with []: %v", attrs.PositiveUploadPermissions)
	}
	if len(*attrs.PositiveItemTypePermissions) != 1 || len(*attrs.NegativeUploadPermissions) != 1 {
		t.Errorf("declared halves must be untouched")
	}

	var absent roleAttributes
	completeContentPermissionPairs(&absent)
	if absent.PositiveItemTypePermissions != nil || absent.NegativeItemTypePermissions != nil ||
		absent.PositiveUploadPermissions != nil || absent.NegativeUploadPermissions != nil {
		t.Errorf("fully absent pairs must stay absent: %+v", absent)
	}
}

// TestRoleAttributesFromModel_DeclaredEmptyContentListsAreSent ensures that
// explicitly declared empty lists are still sent as [] (managed, wholesale
// clear), not omitted.
func TestRoleAttributesFromModel_DeclaredEmptyContentListsAreSent(t *testing.T) {
	ctx := context.Background()

	model := RoleResourceModel{Name: types.StringValue("developer")}
	emptyRoleModelLists(&model)

	attrs, _, diags := roleAttributesFromModel(ctx, &model)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	raw, err := json.Marshal(attrs)
	if err != nil {
		t.Fatalf("marshaling attributes: %v", err)
	}
	var a map[string]any
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatalf("unmarshaling attributes: %v", err)
	}

	for _, field := range []string{
		"positive_item_type_permissions", "negative_item_type_permissions",
		"positive_upload_permissions", "negative_upload_permissions",
	} {
		v, ok := a[field]
		if !ok || v == nil {
			t.Errorf("expected %s to serialize as an empty array, got %v (present=%v)", field, v, ok)
			continue
		}
		if arr := mustAssert[[]any](t, v); len(arr) != 0 {
			t.Errorf("%s length = %d, want 0", field, len(arr))
		}
	}
}

func TestRoleAttributesFromModel_EmptyInheritsSerializesAsEmptyArray(t *testing.T) {
	rels := newRoleRelationships(nil)
	raw, err := json.Marshal(rels)
	if err != nil {
		t.Fatalf("marshaling relationships: %v", err)
	}
	if string(raw) != `{"inherits_permissions_from":{"data":[]}}` {
		t.Errorf("relationships = %s, want empty data array", raw)
	}
}
