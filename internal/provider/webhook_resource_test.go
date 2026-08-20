// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"testing"

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
