// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &RoleResource{}
var _ resource.ResourceWithConfigure = &RoleResource{}
var _ resource.ResourceWithImportState = &RoleResource{}

func NewRoleResource() resource.Resource {
	return &RoleResource{}
}

// RoleResource defines the datocms_role resource implementation.
type RoleResource struct {
	client *DatoCMSClient
}

// RoleResourceModel describes the resource data model.
type RoleResourceModel struct {
	ID                              types.String `tfsdk:"id"`
	Project                         types.String `tfsdk:"project"`
	Name                            types.String `tfsdk:"name"`
	CanEditSite                     types.Bool   `tfsdk:"can_edit_site"`
	CanEditFavicon                  types.Bool   `tfsdk:"can_edit_favicon"`
	CanEditSchema                   types.Bool   `tfsdk:"can_edit_schema"`
	CanManageMenu                   types.Bool   `tfsdk:"can_manage_menu"`
	CanEditEnvironment              types.Bool   `tfsdk:"can_edit_environment"`
	CanPromoteEnvironments          types.Bool   `tfsdk:"can_promote_environments"`
	CanManageEnvironments           types.Bool   `tfsdk:"can_manage_environments"`
	CanManageUsers                  types.Bool   `tfsdk:"can_manage_users"`
	CanManageSharedFilters          types.Bool   `tfsdk:"can_manage_shared_filters"`
	CanManageSearchIndexes          types.Bool   `tfsdk:"can_manage_search_indexes"`
	CanManageUploadCollections      types.Bool   `tfsdk:"can_manage_upload_collections"`
	CanManageBuildTriggers          types.Bool   `tfsdk:"can_manage_build_triggers"`
	CanManageWebhooks               types.Bool   `tfsdk:"can_manage_webhooks"`
	CanManageSSO                    types.Bool   `tfsdk:"can_manage_sso"`
	CanManageWorkflows              types.Bool   `tfsdk:"can_manage_workflows"`
	CanManageAccessTokens           types.Bool   `tfsdk:"can_manage_access_tokens"`
	CanAccessAuditLog               types.Bool   `tfsdk:"can_access_audit_log"`
	CanPerformSiteSearch            types.Bool   `tfsdk:"can_perform_site_search"`
	CanAccessBuildEventsLog         types.Bool   `tfsdk:"can_access_build_events_log"`
	CanAccessSearchIndexEventsLog   types.Bool   `tfsdk:"can_access_search_index_events_log"`
	EnvironmentsAccess              types.String `tfsdk:"environments_access"`
	PositiveItemTypePermissions     types.Set    `tfsdk:"positive_item_type_permissions"`
	NegativeItemTypePermissions     types.Set    `tfsdk:"negative_item_type_permissions"`
	PositiveUploadPermissions       types.Set    `tfsdk:"positive_upload_permissions"`
	NegativeUploadPermissions       types.Set    `tfsdk:"negative_upload_permissions"`
	PositiveBuildTriggerPermissions types.Set    `tfsdk:"positive_build_trigger_permissions"`
	NegativeBuildTriggerPermissions types.Set    `tfsdk:"negative_build_trigger_permissions"`
	PositiveSearchIndexPermissions  types.Set    `tfsdk:"positive_search_index_permissions"`
	NegativeSearchIndexPermissions  types.Set    `tfsdk:"negative_search_index_permissions"`
	InheritsPermissionsFrom         types.List   `tfsdk:"inherits_permissions_from"`
	// TODO: expose meta.final_permissions (read-only effective permission
	// set after resolving inherited roles) as a computed attribute.
}

// itemTypePermissionModel mirrors one element of
// positive/negative_item_type_permissions.
type itemTypePermissionModel struct {
	Action            types.String `tfsdk:"action"`
	Environment       types.String `tfsdk:"environment"`
	ItemType          types.String `tfsdk:"item_type"`
	Workflow          types.String `tfsdk:"workflow"`
	OnStage           types.String `tfsdk:"on_stage"`
	ToStage           types.String `tfsdk:"to_stage"`
	OnCreator         types.String `tfsdk:"on_creator"`
	LocalizationScope types.String `tfsdk:"localization_scope"`
	Locale            types.String `tfsdk:"locale"`
}

// uploadPermissionModel mirrors one element of
// positive/negative_upload_permissions.
type uploadPermissionModel struct {
	Action                 types.String `tfsdk:"action"`
	Environment            types.String `tfsdk:"environment"`
	UploadCollection       types.String `tfsdk:"upload_collection"`
	MoveToUploadCollection types.String `tfsdk:"move_to_upload_collection"`
	OnCreator              types.String `tfsdk:"on_creator"`
	LocalizationScope      types.String `tfsdk:"localization_scope"`
	Locale                 types.String `tfsdk:"locale"`
}

// buildTriggerPermissionModel mirrors one element of
// positive/negative_build_trigger_permissions.
type buildTriggerPermissionModel struct {
	BuildTrigger types.String `tfsdk:"build_trigger"`
}

// searchIndexPermissionModel mirrors one element of
// positive/negative_search_index_permissions.
type searchIndexPermissionModel struct {
	SearchIndex types.String `tfsdk:"search_index"`
}

var itemTypePermissionObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"action":             types.StringType,
	"environment":        types.StringType,
	"item_type":          types.StringType,
	"workflow":           types.StringType,
	"on_stage":           types.StringType,
	"to_stage":           types.StringType,
	"on_creator":         types.StringType,
	"localization_scope": types.StringType,
	"locale":             types.StringType,
}}

var uploadPermissionObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"action":                    types.StringType,
	"environment":               types.StringType,
	"upload_collection":         types.StringType,
	"move_to_upload_collection": types.StringType,
	"on_creator":                types.StringType,
	"localization_scope":        types.StringType,
	"locale":                    types.StringType,
}}

var buildTriggerPermissionObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"build_trigger": types.StringType,
}}

var searchIndexPermissionObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"search_index": types.StringType,
}}

func (r *RoleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func canFlagAttribute(description string) schema.BoolAttribute {
	return schema.BoolAttribute{
		Optional:            true,
		Computed:            true,
		Default:             booldefault.StaticBool(false),
		MarkdownDescription: description,
	}
}

func emptyListDefault(elemType attr.Type) defaults.List {
	return listdefault.StaticValue(types.ListValueMust(elemType, []attr.Value{}))
}

func emptySetDefault(elemType attr.Type) defaults.Set {
	return setdefault.StaticValue(types.SetValueMust(elemType, []attr.Value{}))
}

func itemTypePermissionsAttribute(polarity string) schema.SetNestedAttribute {
	return schema.SetNestedAttribute{
		Optional: true,
		Computed: true,
		PlanModifiers: []planmodifier.Set{
			setplanmodifier.UseStateForUnknown(),
		},
		MarkdownDescription: "Item type (model) permissions " + polarity + " for the role. Negative permissions take precedence over positive ones. When omitted from the configuration the rule set is left untouched: whatever is set in the DatoCMS UI is preserved (useful together with `terraform import`). Declare it, even as `[]`, to have Terraform manage it. UI: Content permissions > \"Records and assets permissions\".",
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"action": schema.StringAttribute{
					Required:            true,
					MarkdownDescription: "Action the permission refers to.",
					Validators: []validator.String{
						stringvalidator.OneOf("all", "read", "create", "update", "publish", "duplicate", "delete", "edit_creator", "take_over", "move_to_stage"),
					},
				},
				"environment": schema.StringAttribute{
					Optional:            true,
					MarkdownDescription: "ID of the environment the permission applies to.",
				},
				"item_type": schema.StringAttribute{
					Optional:            true,
					MarkdownDescription: "ID of the item type (model) the permission applies to. Omit for all models. Mutually exclusive with `workflow`.",
				},
				"workflow": schema.StringAttribute{
					Optional:            true,
					MarkdownDescription: "ID of the workflow: restricts the permission to records associated to it. Mutually exclusive with `item_type`.",
				},
				"on_stage": schema.StringAttribute{
					Optional:            true,
					MarkdownDescription: "Restricts the permission to records that are in this workflow stage.",
				},
				"to_stage": schema.StringAttribute{
					Optional:            true,
					MarkdownDescription: "Restricts stage transitions towards this workflow stage.",
				},
				"on_creator": schema.StringAttribute{
					Optional:            true,
					MarkdownDescription: "Restricts the permission depending on who created the record.",
					Validators: []validator.String{
						stringvalidator.OneOf("anyone", "self", "role"),
					},
				},
				"localization_scope": schema.StringAttribute{
					Optional:            true,
					MarkdownDescription: "Localization scope of the permission.",
					Validators: []validator.String{
						stringvalidator.OneOf("all", "localized", "not_localized"),
					},
				},
				"locale": schema.StringAttribute{
					Optional:            true,
					MarkdownDescription: "Locale the permission applies to. Required when `localization_scope` is `localized`.",
				},
			},
		},
	}
}

func uploadPermissionsAttribute(polarity string) schema.SetNestedAttribute {
	return schema.SetNestedAttribute{
		Optional: true,
		Computed: true,
		PlanModifiers: []planmodifier.Set{
			setplanmodifier.UseStateForUnknown(),
		},
		MarkdownDescription: "Upload permissions " + polarity + " for the role. Negative permissions take precedence over positive ones. When omitted from the configuration the rule set is left untouched: whatever is set in the DatoCMS UI is preserved (useful together with `terraform import`). Declare it, even as `[]`, to have Terraform manage it. UI: Content permissions > \"Records and assets permissions\" (media area).",
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"action": schema.StringAttribute{
					Required:            true,
					MarkdownDescription: "Action the permission refers to.",
					Validators: []validator.String{
						stringvalidator.OneOf("all", "read", "create", "update", "delete", "edit_creator", "replace_asset", "move"),
					},
				},
				"environment": schema.StringAttribute{
					Optional:            true,
					MarkdownDescription: "ID of the environment the permission applies to.",
				},
				"upload_collection": schema.StringAttribute{
					Optional:            true,
					MarkdownDescription: "ID of the upload collection the permission applies to. Omit for all collections.",
				},
				"move_to_upload_collection": schema.StringAttribute{
					Optional:            true,
					MarkdownDescription: "ID of the target upload collection. Only valid when `action` is `move`.",
				},
				"on_creator": schema.StringAttribute{
					Optional:            true,
					MarkdownDescription: "Restricts the permission depending on who created the upload.",
					Validators: []validator.String{
						stringvalidator.OneOf("anyone", "self", "role"),
					},
				},
				"localization_scope": schema.StringAttribute{
					Optional:            true,
					MarkdownDescription: "Localization scope of the permission.",
					Validators: []validator.String{
						stringvalidator.OneOf("all", "localized", "not_localized"),
					},
				},
				"locale": schema.StringAttribute{
					Optional:            true,
					MarkdownDescription: "Locale the permission applies to. Required when `localization_scope` is `localized`.",
				},
			},
		},
	}
}

func buildTriggerPermissionsAttribute(polarity string) schema.SetNestedAttribute {
	return schema.SetNestedAttribute{
		Optional:            true,
		Computed:            true,
		Default:             emptySetDefault(buildTriggerPermissionObjectType),
		MarkdownDescription: "Build triggers the role can " + polarity + " trigger manually. A `null` `build_trigger` covers every build trigger. Creating/editing the triggers themselves is gated by `can_manage_build_triggers`. UI: Build permissions > \"Add new rule\".",
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"build_trigger": schema.StringAttribute{
					Optional:            true,
					MarkdownDescription: "ID of the build trigger. Omit (null) to cover every build trigger.",
				},
			},
		},
	}
}

func searchIndexPermissionsAttribute(polarity string) schema.SetNestedAttribute {
	return schema.SetNestedAttribute{
		Optional:            true,
		Computed:            true,
		Default:             emptySetDefault(searchIndexPermissionObjectType),
		MarkdownDescription: "Search indexes the role can " + polarity + " re-index manually. A `null` `search_index` covers every index. Creating/editing the indexes themselves is gated by `can_manage_search_indexes`. UI: Permissions to index with Search Indexes > \"Add new rule\".",
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"search_index": schema.StringAttribute{
					Optional:            true,
					MarkdownDescription: "ID of the search index. Omit (null) to cover every search index.",
				},
			},
		},
	}
}

func (r *RoleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a DatoCMS role (UI: Settings > Roles). A role bundles the project-level capabilities shown as toggles in the \"Create a new role\" screen (`can_*` flags), the environment access selector, granular record/asset permission rules, manual build trigger and search index permission rules, and role inheritance. Each attribute description below points to the corresponding toggle in the DatoCMS UI. Declared permission arrays are replaced wholesale on update, and negative permissions always take precedence over positive ones. The four item/upload permission lists (`positive_item_type_permissions`, `negative_item_type_permissions`, `positive_upload_permissions`, `negative_upload_permissions`) are preserved as-is on the platform when omitted from the configuration; declare them (even as `[]`) to have Terraform manage them.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Role identifier.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the role.",
			},
			"project":                            projectAttribute(),
			"can_edit_site":                      canFlagAttribute("Whether the role can change project-wide settings (project name, internal subdomain, frontend preview URL, deployment settings). UI: Project and environment management > \"Change project global properties\"."),
			"can_edit_favicon":                   canFlagAttribute("Whether the role can edit favicon, global SEO settings and no-index policy. UI: Environment permissions > \"Edit favicon, global SEO settings and no-index policy\"."),
			"can_edit_schema":                    canFlagAttribute("Whether the role can create and edit the project schema: models, block models, fields, fieldsets, validators and plugins. UI: Environment permissions > \"Create/edit models and plugins\"."),
			"can_manage_menu":                    canFlagAttribute("Whether the role can customize the content navigation bar. UI: Environment permissions > \"Customize all Content navigation items\"."),
			"can_edit_environment":               canFlagAttribute("Whether the role can edit per-environment settings of the environments it has access to: locales, timezone and UI theme. Not about creating or entering environments (see `can_manage_environments` and `environments_access`). UI: Environment permissions > \"Change environment Locales & Timezone, Appearance, and run Available updates\"."),
			"can_promote_environments":           canFlagAttribute("Whether the role can promote a sandbox environment to primary (atomic swap) and toggle the project's maintenance mode. UI: Project and environment management > \"Promote sandbox environments to primary, rename all environments and manage maintenance mode\"."),
			"can_manage_environments":            canFlagAttribute("Whether the role can create, fork and delete sandbox environments. Promotion to primary is gated separately by `can_promote_environments`. UI: Project and environment management > \"Create/delete/rename sandbox environments\"."),
			"can_manage_users":                   canFlagAttribute("Whether the role can create/edit roles and invite/remove collaborators. UI: Project and environment management > \"Create/edit roles and invite/remove collaborators\"."),
			"can_manage_shared_filters":          canFlagAttribute("Whether the role can create and edit shared filters, both for models and the media area. UI: Environment permissions > \"Create/edit shared filters (both for models and the media area)\"."),
			"can_manage_search_indexes":          canFlagAttribute("Whether the role can create and edit search indexes. UI: Automations and integrations > \"Create/edit Search Indexes\"."),
			"can_manage_upload_collections":      canFlagAttribute("Whether the role can create and edit upload collections. UI: Environment permissions > \"Create/edit upload collections\"."),
			"can_manage_build_triggers":          canFlagAttribute("Whether the role can create and edit build triggers. UI: Automations and integrations > \"Create/edit Build triggers\"."),
			"can_manage_webhooks":                canFlagAttribute("Whether the role can create and edit webhooks. UI: Automations and integrations > \"Create/edit webhooks\"."),
			"can_manage_sso":                     canFlagAttribute("Whether the role can manage the Single Sign-On settings. UI: Project and environment management > \"Manage Single Sign-On settings\"."),
			"can_manage_workflows":               canFlagAttribute("Whether the role can create and edit workflows. UI: Environment permissions > \"Create/edit workflows\"."),
			"can_manage_access_tokens":           canFlagAttribute("Whether the role can manage API tokens. UI: Automations and integrations > \"Create/edit API tokens\"."),
			"can_access_audit_log":               canFlagAttribute("Whether the role can access the Audit Log. UI: Activity tracking > \"Access to Audit log events\"."),
			"can_perform_site_search":            canFlagAttribute("Whether the role can perform Site Search API calls. UI: Automations and integrations > \"Perform Site Search API calls\"."),
			"can_access_build_events_log":        canFlagAttribute("Whether the role can access the build events log. UI: Activity tracking > \"Access deployment activity log\"."),
			"can_access_search_index_events_log": canFlagAttribute("Whether the role can access the search index events log. UI: Activity tracking > \"Access Search index activity log\"."),
			"environments_access": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Specifies the environments the user can access: `all`, `primary_only`, `sandbox_only` or `none`. `none` is typically used when the role inherits environment access from other roles. The API default is not documented; when omitted the value returned by DatoCMS is stored. UI: Environment access, the two toggles \"On primary environment\" and \"On sandbox environments\":\n\n| On primary environment | On sandbox environments | Value |\n|---|---|---|\n| on | on | `all` |\n| on | off | `primary_only` |\n| off | on | `sandbox_only` |\n| off | off | `none` |",
				Validators: []validator.String{
					stringvalidator.OneOf("all", "primary_only", "sandbox_only", "none"),
				},
			},
			"positive_item_type_permissions":     itemTypePermissionsAttribute("granted"),
			"negative_item_type_permissions":     itemTypePermissionsAttribute("denied"),
			"positive_upload_permissions":        uploadPermissionsAttribute("granted"),
			"negative_upload_permissions":        uploadPermissionsAttribute("denied"),
			"positive_build_trigger_permissions": buildTriggerPermissionsAttribute(""),
			"negative_build_trigger_permissions": buildTriggerPermissionsAttribute("NOT"),
			"positive_search_index_permissions":  searchIndexPermissionsAttribute(""),
			"negative_search_index_permissions":  searchIndexPermissionsAttribute("NOT"),
			"inherits_permissions_from": schema.ListAttribute{
				Optional:            true,
				Computed:            true,
				Default:             emptyListDefault(types.StringType),
				ElementType:         types.StringType,
				MarkdownDescription: "IDs of the roles this role inherits permissions from. Inherited permissions are unioned with the role's own. UI: \"Inherits permissions from\" select.",
			},
			// TODO: expose meta.final_permissions (read-only) once needed.
		},
	}
}

func (r *RoleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func stringPtr(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return v.ValueStringPointer()
}

func stringFromPtr(p *string) types.String {
	if p == nil {
		return types.StringNull()
	}
	return types.StringValue(*p)
}

// roleAttributesFromModel converts the Terraform plan/state model into the
// JSON:API attribute set plus the list of inherited role IDs. Every attribute
// is sent except the four item/upload permission lists when they are null or
// unknown in the model: those stay nil so the request omits them and DatoCMS
// leaves them unchanged (declared lists, including empty ones, are sent and
// replaced wholesale).
func roleAttributesFromModel(ctx context.Context, data *RoleResourceModel) (roleAttributes, []string, diag.Diagnostics) {
	var diags diag.Diagnostics

	attrs := roleAttributes{
		Name:                          data.Name.ValueString(),
		CanEditSite:                   data.CanEditSite.ValueBool(),
		CanEditFavicon:                data.CanEditFavicon.ValueBool(),
		CanEditSchema:                 data.CanEditSchema.ValueBool(),
		CanManageMenu:                 data.CanManageMenu.ValueBool(),
		CanEditEnvironment:            data.CanEditEnvironment.ValueBool(),
		CanPromoteEnvironments:        data.CanPromoteEnvironments.ValueBool(),
		CanManageEnvironments:         data.CanManageEnvironments.ValueBool(),
		CanManageUsers:                data.CanManageUsers.ValueBool(),
		CanManageSharedFilters:        data.CanManageSharedFilters.ValueBool(),
		CanManageSearchIndexes:        data.CanManageSearchIndexes.ValueBool(),
		CanManageUploadCollections:    data.CanManageUploadCollections.ValueBool(),
		CanManageBuildTriggers:        data.CanManageBuildTriggers.ValueBool(),
		CanManageWebhooks:             data.CanManageWebhooks.ValueBool(),
		CanManageSSO:                  data.CanManageSSO.ValueBool(),
		CanManageWorkflows:            data.CanManageWorkflows.ValueBool(),
		CanManageAccessTokens:         data.CanManageAccessTokens.ValueBool(),
		CanAccessAuditLog:             data.CanAccessAuditLog.ValueBool(),
		CanPerformSiteSearch:          data.CanPerformSiteSearch.ValueBool(),
		CanAccessBuildEventsLog:       data.CanAccessBuildEventsLog.ValueBool(),
		CanAccessSearchIndexEventsLog: data.CanAccessSearchIndexEventsLog.ValueBool(),
		EnvironmentsAccess:            stringPtr(data.EnvironmentsAccess),
	}

	if isKnownSet(data.PositiveItemTypePermissions) {
		var itemTypeModels []itemTypePermissionModel
		diags.Append(data.PositiveItemTypePermissions.ElementsAs(ctx, &itemTypeModels, false)...)
		perms := itemTypePermissionsFromModels(itemTypeModels)
		attrs.PositiveItemTypePermissions = &perms
	}

	if isKnownSet(data.NegativeItemTypePermissions) {
		var itemTypeModels []itemTypePermissionModel
		diags.Append(data.NegativeItemTypePermissions.ElementsAs(ctx, &itemTypeModels, false)...)
		perms := itemTypePermissionsFromModels(itemTypeModels)
		attrs.NegativeItemTypePermissions = &perms
	}

	if isKnownSet(data.PositiveUploadPermissions) {
		var uploadModels []uploadPermissionModel
		diags.Append(data.PositiveUploadPermissions.ElementsAs(ctx, &uploadModels, false)...)
		perms := uploadPermissionsFromModels(uploadModels)
		attrs.PositiveUploadPermissions = &perms
	}

	if isKnownSet(data.NegativeUploadPermissions) {
		var uploadModels []uploadPermissionModel
		diags.Append(data.NegativeUploadPermissions.ElementsAs(ctx, &uploadModels, false)...)
		perms := uploadPermissionsFromModels(uploadModels)
		attrs.NegativeUploadPermissions = &perms
	}

	var buildTriggerModels []buildTriggerPermissionModel
	diags.Append(data.PositiveBuildTriggerPermissions.ElementsAs(ctx, &buildTriggerModels, false)...)
	attrs.PositiveBuildTriggerPermissions = buildTriggerPermissionsFromModels(buildTriggerModels)

	buildTriggerModels = nil
	diags.Append(data.NegativeBuildTriggerPermissions.ElementsAs(ctx, &buildTriggerModels, false)...)
	attrs.NegativeBuildTriggerPermissions = buildTriggerPermissionsFromModels(buildTriggerModels)

	var searchIndexModels []searchIndexPermissionModel
	diags.Append(data.PositiveSearchIndexPermissions.ElementsAs(ctx, &searchIndexModels, false)...)
	attrs.PositiveSearchIndexPermissions = searchIndexPermissionsFromModels(searchIndexModels)

	searchIndexModels = nil
	diags.Append(data.NegativeSearchIndexPermissions.ElementsAs(ctx, &searchIndexModels, false)...)
	attrs.NegativeSearchIndexPermissions = searchIndexPermissionsFromModels(searchIndexModels)

	var inheritsFrom []string
	diags.Append(data.InheritsPermissionsFrom.ElementsAs(ctx, &inheritsFrom, false)...)

	return attrs, inheritsFrom, diags
}

func itemTypePermissionsFromModels(models []itemTypePermissionModel) []roleItemTypePermission {
	out := make([]roleItemTypePermission, 0, len(models))
	for _, m := range models {
		out = append(out, roleItemTypePermission{
			Action:            m.Action.ValueString(),
			Environment:       stringPtr(m.Environment),
			ItemType:          stringPtr(m.ItemType),
			Workflow:          stringPtr(m.Workflow),
			OnStage:           stringPtr(m.OnStage),
			ToStage:           stringPtr(m.ToStage),
			OnCreator:         stringPtr(m.OnCreator),
			LocalizationScope: stringPtr(m.LocalizationScope),
			Locale:            stringPtr(m.Locale),
		})
	}
	return out
}

func uploadPermissionsFromModels(models []uploadPermissionModel) []roleUploadPermission {
	out := make([]roleUploadPermission, 0, len(models))
	for _, m := range models {
		out = append(out, roleUploadPermission{
			Action:                 m.Action.ValueString(),
			Environment:            stringPtr(m.Environment),
			UploadCollection:       stringPtr(m.UploadCollection),
			MoveToUploadCollection: stringPtr(m.MoveToUploadCollection),
			OnCreator:              stringPtr(m.OnCreator),
			LocalizationScope:      stringPtr(m.LocalizationScope),
			Locale:                 stringPtr(m.Locale),
		})
	}
	return out
}

func buildTriggerPermissionsFromModels(models []buildTriggerPermissionModel) []roleBuildTriggerPermission {
	out := make([]roleBuildTriggerPermission, 0, len(models))
	for _, m := range models {
		out = append(out, roleBuildTriggerPermission{BuildTrigger: stringPtr(m.BuildTrigger)})
	}
	return out
}

func searchIndexPermissionsFromModels(models []searchIndexPermissionModel) []roleSearchIndexPermission {
	out := make([]roleSearchIndexPermission, 0, len(models))
	for _, m := range models {
		out = append(out, roleSearchIndexPermission{SearchIndex: stringPtr(m.SearchIndex)})
	}
	return out
}

// isKnownSet reports whether a set attribute carries a concrete value
// (declared in configuration or resolved from state), as opposed to being
// null (omitted) or unknown (omitted on create).
func isKnownSet(s types.Set) bool {
	return !s.IsNull() && !s.IsUnknown()
}

// completeContentPermissionPairs fills the missing half of a partially
// declared item/upload permission pair with an empty list. The DatoCMS API
// requires both halves of each positive/negative pair to be both present or
// both absent (POSITIVE_AND_NEGATIVE_ITEM_TYPE_PERMISSIONS_MUST_BE_BOTH_
// PRESENT_OR_BOTH_ABSENT). Used on create, where there is nothing to
// preserve yet.
func completeContentPermissionPairs(attrs *roleAttributes) {
	if (attrs.PositiveItemTypePermissions == nil) != (attrs.NegativeItemTypePermissions == nil) {
		empty := []roleItemTypePermission{}
		if attrs.PositiveItemTypePermissions == nil {
			attrs.PositiveItemTypePermissions = &empty
		} else {
			attrs.NegativeItemTypePermissions = &empty
		}
	}
	if (attrs.PositiveUploadPermissions == nil) != (attrs.NegativeUploadPermissions == nil) {
		empty := []roleUploadPermission{}
		if attrs.PositiveUploadPermissions == nil {
			attrs.PositiveUploadPermissions = &empty
		} else {
			attrs.NegativeUploadPermissions = &empty
		}
	}
}

// derefPermissions turns the API's pointer-to-slice permission lists back
// into plain slices (nil pointer means an empty list in responses).
func derefPermissions[T any](p *[]T) []T {
	if p == nil {
		return nil
	}
	return *p
}

func itemTypeModelsFromAPI(perms []roleItemTypePermission) []itemTypePermissionModel {
	out := make([]itemTypePermissionModel, 0, len(perms))
	for _, p := range perms {
		out = append(out, itemTypePermissionModel{
			Action:            types.StringValue(p.Action),
			Environment:       stringFromPtr(p.Environment),
			ItemType:          stringFromPtr(p.ItemType),
			Workflow:          stringFromPtr(p.Workflow),
			OnStage:           stringFromPtr(p.OnStage),
			ToStage:           stringFromPtr(p.ToStage),
			OnCreator:         stringFromPtr(p.OnCreator),
			LocalizationScope: stringFromPtr(p.LocalizationScope),
			Locale:            stringFromPtr(p.Locale),
		})
	}
	return out
}

func uploadModelsFromAPI(perms []roleUploadPermission) []uploadPermissionModel {
	out := make([]uploadPermissionModel, 0, len(perms))
	for _, p := range perms {
		out = append(out, uploadPermissionModel{
			Action:                 types.StringValue(p.Action),
			Environment:            stringFromPtr(p.Environment),
			UploadCollection:       stringFromPtr(p.UploadCollection),
			MoveToUploadCollection: stringFromPtr(p.MoveToUploadCollection),
			OnCreator:              stringFromPtr(p.OnCreator),
			LocalizationScope:      stringFromPtr(p.LocalizationScope),
			Locale:                 stringFromPtr(p.Locale),
		})
	}
	return out
}

// modelFromRole maps a role API response onto the resource model.
func modelFromRole(ctx context.Context, role *roleData, data *RoleResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	data.ID = types.StringValue(role.ID)
	a := role.Attributes
	data.Name = types.StringValue(a.Name)
	data.CanEditSite = types.BoolValue(a.CanEditSite)
	data.CanEditFavicon = types.BoolValue(a.CanEditFavicon)
	data.CanEditSchema = types.BoolValue(a.CanEditSchema)
	data.CanManageMenu = types.BoolValue(a.CanManageMenu)
	data.CanEditEnvironment = types.BoolValue(a.CanEditEnvironment)
	data.CanPromoteEnvironments = types.BoolValue(a.CanPromoteEnvironments)
	data.CanManageEnvironments = types.BoolValue(a.CanManageEnvironments)
	data.CanManageUsers = types.BoolValue(a.CanManageUsers)
	data.CanManageSharedFilters = types.BoolValue(a.CanManageSharedFilters)
	data.CanManageSearchIndexes = types.BoolValue(a.CanManageSearchIndexes)
	data.CanManageUploadCollections = types.BoolValue(a.CanManageUploadCollections)
	data.CanManageBuildTriggers = types.BoolValue(a.CanManageBuildTriggers)
	data.CanManageWebhooks = types.BoolValue(a.CanManageWebhooks)
	data.CanManageSSO = types.BoolValue(a.CanManageSSO)
	data.CanManageWorkflows = types.BoolValue(a.CanManageWorkflows)
	data.CanManageAccessTokens = types.BoolValue(a.CanManageAccessTokens)
	data.CanAccessAuditLog = types.BoolValue(a.CanAccessAuditLog)
	data.CanPerformSiteSearch = types.BoolValue(a.CanPerformSiteSearch)
	data.CanAccessBuildEventsLog = types.BoolValue(a.CanAccessBuildEventsLog)
	data.CanAccessSearchIndexEventsLog = types.BoolValue(a.CanAccessSearchIndexEventsLog)
	data.EnvironmentsAccess = stringFromPtr(a.EnvironmentsAccess)

	buildTriggerModelsFor := func(perms []roleBuildTriggerPermission) []buildTriggerPermissionModel {
		out := make([]buildTriggerPermissionModel, 0, len(perms))
		for _, p := range perms {
			out = append(out, buildTriggerPermissionModel{BuildTrigger: stringFromPtr(p.BuildTrigger)})
		}
		return out
	}

	searchIndexModelsFor := func(perms []roleSearchIndexPermission) []searchIndexPermissionModel {
		out := make([]searchIndexPermissionModel, 0, len(perms))
		for _, p := range perms {
			out = append(out, searchIndexPermissionModel{SearchIndex: stringFromPtr(p.SearchIndex)})
		}
		return out
	}

	var d diag.Diagnostics
	data.PositiveItemTypePermissions, d = types.SetValueFrom(ctx, itemTypePermissionObjectType, itemTypeModelsFromAPI(derefPermissions(a.PositiveItemTypePermissions)))
	diags.Append(d...)
	data.NegativeItemTypePermissions, d = types.SetValueFrom(ctx, itemTypePermissionObjectType, itemTypeModelsFromAPI(derefPermissions(a.NegativeItemTypePermissions)))
	diags.Append(d...)
	data.PositiveUploadPermissions, d = types.SetValueFrom(ctx, uploadPermissionObjectType, uploadModelsFromAPI(derefPermissions(a.PositiveUploadPermissions)))
	diags.Append(d...)
	data.NegativeUploadPermissions, d = types.SetValueFrom(ctx, uploadPermissionObjectType, uploadModelsFromAPI(derefPermissions(a.NegativeUploadPermissions)))
	diags.Append(d...)
	data.PositiveBuildTriggerPermissions, d = types.SetValueFrom(ctx, buildTriggerPermissionObjectType, buildTriggerModelsFor(a.PositiveBuildTriggerPermissions))
	diags.Append(d...)
	data.NegativeBuildTriggerPermissions, d = types.SetValueFrom(ctx, buildTriggerPermissionObjectType, buildTriggerModelsFor(a.NegativeBuildTriggerPermissions))
	diags.Append(d...)
	data.PositiveSearchIndexPermissions, d = types.SetValueFrom(ctx, searchIndexPermissionObjectType, searchIndexModelsFor(a.PositiveSearchIndexPermissions))
	diags.Append(d...)
	data.NegativeSearchIndexPermissions, d = types.SetValueFrom(ctx, searchIndexPermissionObjectType, searchIndexModelsFor(a.NegativeSearchIndexPermissions))
	diags.Append(d...)

	inheritsFrom := []string{}
	if role.Relationships != nil {
		for _, linkage := range role.Relationships.InheritsPermissionsFrom.Data {
			inheritsFrom = append(inheritsFrom, linkage.ID)
		}
	}
	data.InheritsPermissionsFrom, d = types.ListValueFrom(ctx, types.StringType, inheritsFrom)
	diags.Append(d...)

	return diags
}

// --- CRUD ---

func (r *RoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RoleResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	attrs, inheritsFrom, diags := roleAttributesFromModel(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The API rejects a partially declared item/upload pair; on create the
	// undeclared half becomes [] (a brand new role has nothing to preserve).
	completeContentPermissionPairs(&attrs)

	client := clientForProject(r.client, data.Project, &resp.Diagnostics)
	if client == nil {
		return
	}

	role, err := client.CreateRole(ctx, attrs, inheritsFrom)
	if err != nil {
		resp.Diagnostics.AddError("Error creating DatoCMS role", err.Error())
		return
	}

	data.ID = types.StringValue(role.ID)
	// environments_access has no documented API default: when omitted from
	// the configuration its planned value is unknown, so store whatever the
	// API returned. The same goes for the four item/upload permission lists:
	// when omitted they are unknown in the plan (they were not sent in the
	// request), so the state takes whatever the API returned. All other
	// attributes keep the planned values (defaults make them concrete, and
	// the API accepts arrays wholesale).
	if data.EnvironmentsAccess.IsUnknown() {
		data.EnvironmentsAccess = stringFromPtr(role.Attributes.EnvironmentsAccess)
	}

	var d diag.Diagnostics
	if data.PositiveItemTypePermissions.IsUnknown() {
		data.PositiveItemTypePermissions, d = types.SetValueFrom(ctx, itemTypePermissionObjectType, itemTypeModelsFromAPI(derefPermissions(role.Attributes.PositiveItemTypePermissions)))
		resp.Diagnostics.Append(d...)
	}
	if data.NegativeItemTypePermissions.IsUnknown() {
		data.NegativeItemTypePermissions, d = types.SetValueFrom(ctx, itemTypePermissionObjectType, itemTypeModelsFromAPI(derefPermissions(role.Attributes.NegativeItemTypePermissions)))
		resp.Diagnostics.Append(d...)
	}
	if data.PositiveUploadPermissions.IsUnknown() {
		data.PositiveUploadPermissions, d = types.SetValueFrom(ctx, uploadPermissionObjectType, uploadModelsFromAPI(derefPermissions(role.Attributes.PositiveUploadPermissions)))
		resp.Diagnostics.Append(d...)
	}
	if data.NegativeUploadPermissions.IsUnknown() {
		data.NegativeUploadPermissions, d = types.SetValueFrom(ctx, uploadPermissionObjectType, uploadModelsFromAPI(derefPermissions(role.Attributes.NegativeUploadPermissions)))
		resp.Diagnostics.Append(d...)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RoleResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client := clientForProject(r.client, data.Project, &resp.Diagnostics)
	if client == nil {
		return
	}

	role, err := client.GetRole(ctx, data.ID.ValueString())
	if err != nil {
		if errors.Is(err, errNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading DatoCMS role", err.Error())
		return
	}

	resp.Diagnostics.Append(modelFromRole(ctx, role, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data, config RoleResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	attrs, inheritsFrom, diags := roleAttributesFromModel(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Item/upload permission lists omitted from the configuration are not
	// sent on update either: the DatoCMS API leaves omitted attributes
	// unchanged, so whatever the platform has is preserved. The API requires
	// both halves of each positive/negative pair to be both present or both
	// absent, so a pair is omitted only when BOTH halves are undeclared;
	// when only one half is declared, the other is sent with its planned
	// value, which comes from state via UseStateForUnknown (the last
	// refreshed platform value, i.e. still effectively preserved).
	if config.PositiveItemTypePermissions.IsNull() && config.NegativeItemTypePermissions.IsNull() {
		attrs.PositiveItemTypePermissions = nil
		attrs.NegativeItemTypePermissions = nil
	}
	if config.PositiveUploadPermissions.IsNull() && config.NegativeUploadPermissions.IsNull() {
		attrs.PositiveUploadPermissions = nil
		attrs.NegativeUploadPermissions = nil
	}

	client := clientForProject(r.client, data.Project, &resp.Diagnostics)
	if client == nil {
		return
	}

	role, err := client.UpdateRole(ctx, data.ID.ValueString(), attrs, inheritsFrom)
	if err != nil {
		resp.Diagnostics.AddError("Error updating DatoCMS role", err.Error())
		return
	}

	if data.EnvironmentsAccess.IsUnknown() {
		data.EnvironmentsAccess = stringFromPtr(role.Attributes.EnvironmentsAccess)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RoleResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client := clientForProject(r.client, data.Project, &resp.Diagnostics)
	if client == nil {
		return
	}

	if err := client.DeleteRole(ctx, data.ID.ValueString()); err != nil {
		if errors.Is(err, errNotFound) {
			return
		}
		resp.Diagnostics.AddError("Error deleting DatoCMS role", err.Error())
	}
}

func (r *RoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importStateWithProject(ctx, req, resp)
}
