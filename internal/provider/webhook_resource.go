// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &WebhookResource{}
var _ resource.ResourceWithConfigure = &WebhookResource{}
var _ resource.ResourceWithImportState = &WebhookResource{}

func NewWebhookResource() resource.Resource {
	return &WebhookResource{}
}

// WebhookResource defines the datocms_webhook resource implementation.
type WebhookResource struct {
	client *DatoCMSClient
}

// WebhookResourceModel describes the resource data model.
type WebhookResourceModel struct {
	ID                   types.String `tfsdk:"id"`
	Name                 types.String `tfsdk:"name"`
	URL                  types.String `tfsdk:"url"`
	Headers              types.Map    `tfsdk:"headers"`
	Events               types.List   `tfsdk:"events"`
	CustomPayload        types.String `tfsdk:"custom_payload"`
	HTTPBasicUser        types.String `tfsdk:"http_basic_user"`
	HTTPBasicPassword    types.String `tfsdk:"http_basic_password"`
	Enabled              types.Bool   `tfsdk:"enabled"`
	PayloadAPIVersion    types.String `tfsdk:"payload_api_version"`
	NestedItemsInPayload types.Bool   `tfsdk:"nested_items_in_payload"`
	AutoRetry            types.Bool   `tfsdk:"auto_retry"`
}

// webhookEventModel mirrors one element of events.
type webhookEventModel struct {
	EntityType types.String `tfsdk:"entity_type"`
	EventTypes types.List   `tfsdk:"event_types"`
	Filters    types.List   `tfsdk:"filters"`
}

// webhookEventFilterModel mirrors one element of events[].filters.
type webhookEventFilterModel struct {
	EntityType types.String `tfsdk:"entity_type"`
	EntityIDs  types.List   `tfsdk:"entity_ids"`
}

var webhookEventFilterObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"entity_type": types.StringType,
	"entity_ids":  types.ListType{ElemType: types.StringType},
}}

var webhookEventObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"entity_type": types.StringType,
	"event_types": types.ListType{ElemType: types.StringType},
	"filters":     types.ListType{ElemType: webhookEventFilterObjectType},
}}

// Enums from the CMA hyperschema (definitions.webhook).
var (
	webhookEventEntityTypes = []string{
		"item_type", "item", "upload", "build_trigger",
		"environment", "maintenance_mode", "sso_user", "cda_cache_tags",
	}
	webhookEventTypes = []string{
		"create", "update", "delete", "publish", "unpublish", "promote",
		"deploy_started", "deploy_succeeded", "deploy_failed", "change", "invalidate",
	}
	webhookFilterEntityTypes = []string{
		"item_type", "item", "build_trigger", "environment", "environment_type",
	}
)

func (r *WebhookResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhook"
}

func (r *WebhookResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a DatoCMS webhook (UI: Settings > Webhooks > \"Create a new webhook\"). A webhook calls the configured URL whenever the selected events happen in the project; the payload can be customized with Mustache templates. Each attribute description below points to the corresponding field of the webhook form, which is organized in three sections: Triggers, HTTP Settings and HTTP Body, plus the general fields at the top (\"Name\", \"Enabled?\", \"Automatic retries?\").",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Webhook identifier.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Unique name of the webhook. UI: \"Name\".",
			},
			"url": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "URL to call. Mustache tags are supported for dynamic destinations. UI: HTTP Settings > \"URL\".",
			},
			"headers": schema.MapAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				Default:             mapdefault.StaticValue(types.MapValueMust(types.StringType, map[string]attr.Value{})),
				MarkdownDescription: "Additional headers that will be sent with the webhook call. UI: HTTP Settings > \"Custom headers\" > \"Add new header\".",
			},
			"events": schema.ListNestedAttribute{
				Required:            true,
				MarkdownDescription: "Events that trigger the webhook. Each element corresponds to one trigger in the UI. UI: Triggers > \"Add new trigger\".",
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"entity_type": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The subject of webhook triggering.",
							Validators: []validator.String{
								stringvalidator.OneOf(webhookEventEntityTypes...),
							},
						},
						"event_types": schema.ListAttribute{
							Required:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "Types of events that trigger the webhook for the entity.",
							Validators: []validator.List{
								listvalidator.SizeAtLeast(1),
								listvalidator.ValueStringsAre(
									stringvalidator.OneOf(webhookEventTypes...),
								),
							},
						},
						"filters": schema.ListNestedAttribute{
							Optional:            true,
							MarkdownDescription: "Conditions restricting when the webhook triggers.",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"entity_type": schema.StringAttribute{
										Required:            true,
										MarkdownDescription: "Type of entity the filter refers to.",
										Validators: []validator.String{
											stringvalidator.OneOf(webhookFilterEntityTypes...),
										},
									},
									"entity_ids": schema.ListAttribute{
										Required:            true,
										ElementType:         types.StringType,
										MarkdownDescription: "IDs of the entities the filter refers to.",
										Validators: []validator.List{
											listvalidator.SizeAtLeast(1),
										},
									},
								},
							},
						},
					},
				},
			},
			"custom_payload": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Custom payload for the webhook call; Mustache template syntax is supported, rendered against the default webhook payload. If the result is not JSON, set an appropriate `Content-Type` in `headers`. UI: HTTP Body > \"Send a custom payload?\": setting this attribute is equivalent to turning the toggle on; a null/omitted attribute means the toggle is off.",
			},
			"http_basic_user": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "HTTP Basic Authorization username. UI: HTTP Settings > \"HTTP basic auth\" > \"User\".",
			},
			"http_basic_password": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "HTTP Basic Authorization password. UI: HTTP Settings > \"HTTP basic auth\" > \"Password\".",
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Whether the webhook is enabled and sending events or not. UI: \"Enabled?\" toggle, on by default.",
			},
			"payload_api_version": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("3"),
				MarkdownDescription: "API version used when serializing entities in the payload. UI: HTTP Body > \"Payload format\" select, where `\"3\"` corresponds to \"API Version 3\" (the UI default).",
			},
			"nested_items_in_payload": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Whether the records present in the payload show blocks expanded or not. UI: HTTP Body > \"Expand nested blocks in records?\" toggle.",
			},
			"auto_retry": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "If enabled, the system will attempt to retry the call several times in the event of timeouts or errors (the UI notes delivery is retried up to 7 times). UI: \"Automatic retries?\" toggle.",
			},
		},
	}
}

func (r *WebhookResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*DatoCMSClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *DatoCMSClient, got: %T.", req.ProviderData),
		)
		return
	}

	r.client = client
}

// --- model <-> API mapping ---

// webhookAttributesFromModel converts the Terraform plan/state model into the
// full JSON:API attribute set. The nullable create-required fields
// (custom_payload, http_basic_user, http_basic_password) map to nil pointers
// when omitted, which serialize as explicit nulls.
func webhookAttributesFromModel(ctx context.Context, data *WebhookResourceModel) (webhookAttributes, diag.Diagnostics) {
	var diags diag.Diagnostics

	attrs := webhookAttributes{
		Name:                 data.Name.ValueString(),
		URL:                  data.URL.ValueString(),
		CustomPayload:        stringPtr(data.CustomPayload),
		HTTPBasicUser:        stringPtr(data.HTTPBasicUser),
		HTTPBasicPassword:    stringPtr(data.HTTPBasicPassword),
		Enabled:              data.Enabled.ValueBool(),
		PayloadAPIVersion:    data.PayloadAPIVersion.ValueString(),
		NestedItemsInPayload: data.NestedItemsInPayload.ValueBool(),
		AutoRetry:            data.AutoRetry.ValueBool(),
	}

	headers := map[string]string{}
	if !data.Headers.IsNull() && !data.Headers.IsUnknown() {
		diags.Append(data.Headers.ElementsAs(ctx, &headers, false)...)
	}
	attrs.Headers = headers

	var eventModels []webhookEventModel
	diags.Append(data.Events.ElementsAs(ctx, &eventModels, false)...)
	if diags.HasError() {
		return attrs, diags
	}

	attrs.Events = make([]webhookEvent, 0, len(eventModels))
	for _, em := range eventModels {
		event := webhookEvent{EntityType: em.EntityType.ValueString()}

		diags.Append(em.EventTypes.ElementsAs(ctx, &event.EventTypes, false)...)

		if !em.Filters.IsNull() && !em.Filters.IsUnknown() {
			var filterModels []webhookEventFilterModel
			diags.Append(em.Filters.ElementsAs(ctx, &filterModels, false)...)
			for _, fm := range filterModels {
				filter := webhookEventFilter{EntityType: fm.EntityType.ValueString()}
				diags.Append(fm.EntityIDs.ElementsAs(ctx, &filter.EntityIDs, false)...)
				event.Filters = append(event.Filters, filter)
			}
		}

		attrs.Events = append(attrs.Events, event)
	}

	return attrs, diags
}

// modelFromWebhook maps a webhook API response onto the resource model. The
// stored http_basic_password is preserved when the API returns null for it
// (standard drift protection for secrets), since responses are not guaranteed
// to echo the secret back.
func modelFromWebhook(ctx context.Context, webhook *webhookData, data *WebhookResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	var d diag.Diagnostics

	data.ID = types.StringValue(webhook.ID)
	a := webhook.Attributes
	data.Name = types.StringValue(a.Name)
	data.URL = types.StringValue(a.URL)
	data.CustomPayload = stringFromPtr(a.CustomPayload)
	data.HTTPBasicUser = stringFromPtr(a.HTTPBasicUser)
	if a.HTTPBasicPassword != nil || data.HTTPBasicPassword.IsUnknown() {
		data.HTTPBasicPassword = stringFromPtr(a.HTTPBasicPassword)
	}
	data.Enabled = types.BoolValue(a.Enabled)
	data.PayloadAPIVersion = types.StringValue(a.PayloadAPIVersion)
	data.NestedItemsInPayload = types.BoolValue(a.NestedItemsInPayload)
	data.AutoRetry = types.BoolValue(a.AutoRetry)

	headers := a.Headers
	if headers == nil {
		headers = map[string]string{}
	}
	data.Headers, d = types.MapValueFrom(ctx, types.StringType, headers)
	diags.Append(d...)

	eventModels := make([]webhookEventModel, 0, len(a.Events))
	for _, event := range a.Events {
		em := webhookEventModel{EntityType: types.StringValue(event.EntityType)}

		em.EventTypes, d = types.ListValueFrom(ctx, types.StringType, event.EventTypes)
		diags.Append(d...)

		// A missing/empty filters array maps to null, matching an omitted
		// optional attribute in the configuration.
		if len(event.Filters) == 0 {
			em.Filters = types.ListNull(webhookEventFilterObjectType)
		} else {
			filterModels := make([]webhookEventFilterModel, 0, len(event.Filters))
			for _, filter := range event.Filters {
				fm := webhookEventFilterModel{EntityType: types.StringValue(filter.EntityType)}
				fm.EntityIDs, d = types.ListValueFrom(ctx, types.StringType, filter.EntityIDs)
				diags.Append(d...)
				filterModels = append(filterModels, fm)
			}
			em.Filters, d = types.ListValueFrom(ctx, webhookEventFilterObjectType, filterModels)
			diags.Append(d...)
		}

		eventModels = append(eventModels, em)
	}
	data.Events, d = types.ListValueFrom(ctx, webhookEventObjectType, eventModels)
	diags.Append(d...)

	return diags
}

// --- CRUD ---

func (r *WebhookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data WebhookResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	attrs, diags := webhookAttributesFromModel(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	webhook, err := r.client.CreateWebhook(ctx, attrs)
	if err != nil {
		resp.Diagnostics.AddError("Error creating DatoCMS webhook", err.Error())
		return
	}

	// All attributes are concrete in the plan (defaults cover the computed
	// ones), so only the server-assigned ID is taken from the response.
	data.ID = types.StringValue(webhook.ID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WebhookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data WebhookResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	webhook, err := r.client.GetWebhook(ctx, data.ID.ValueString())
	if err != nil {
		if errors.Is(err, errNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading DatoCMS webhook", err.Error())
		return
	}

	resp.Diagnostics.Append(modelFromWebhook(ctx, webhook, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WebhookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data WebhookResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	attrs, diags := webhookAttributesFromModel(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.UpdateWebhook(ctx, data.ID.ValueString(), attrs); err != nil {
		resp.Diagnostics.AddError("Error updating DatoCMS webhook", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WebhookResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data WebhookResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteWebhook(ctx, data.ID.ValueString()); err != nil {
		if errors.Is(err, errNotFound) {
			return
		}
		resp.Diagnostics.AddError("Error deleting DatoCMS webhook", err.Error())
	}
}

func (r *WebhookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
