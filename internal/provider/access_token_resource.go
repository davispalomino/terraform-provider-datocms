// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &AccessTokenResource{}
var _ resource.ResourceWithConfigure = &AccessTokenResource{}
var _ resource.ResourceWithImportState = &AccessTokenResource{}

func NewAccessTokenResource() resource.Resource {
	return &AccessTokenResource{}
}

// AccessTokenResource defines the datocms_access_token resource implementation.
type AccessTokenResource struct {
	client *DatoCMSClient
}

// AccessTokenResourceModel describes the resource data model.
type AccessTokenResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	RoleID              types.String `tfsdk:"role_id"`
	CanAccessCda        types.Bool   `tfsdk:"can_access_cda"`
	CanAccessCdaPreview types.Bool   `tfsdk:"can_access_cda_preview"`
	CanAccessCma        types.Bool   `tfsdk:"can_access_cma"`
	Token               types.String `tfsdk:"token"`
	// TODO: expose the read-only usage attributes (last_cma_access,
	// last_cda_access, enum: today/yesterday/this_week/last_week/this_month/
	// last_month/never) and hardcoded_type (factory-token marker, always null
	// for tokens created via the API) as computed attributes once needed.
}

func (r *AccessTokenResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_access_token"
}

func (r *AccessTokenResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a DatoCMS API token (UI: Project Settings > API Tokens > \"New API Token\"; the CMA calls the resource `access_token`). A token combines a role (which actions are permitted) with the API surface flags of the Permissions section (`can_access_cda`, `can_access_cda_preview`, `can_access_cma`); the effective capabilities are the intersection of the two layers. The token secret is returned by the API and stored in the Terraform state as the sensitive `token` attribute.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Access token identifier.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the API token. UI: \"Name\".",
			},
			"role_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "ID of the role associated with the token, typically referencing a `datocms_role` resource. Required: the CMA rejects a null role linkage on both create and update (422 INVALID_FORMAT), so every token must reference a role. The token's effective permissions are the intersection of this role and the `can_access_*` surface flags. UI: Permissions > \"Role associated with this API token\" select.",
			},
			"can_access_cda": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Whether the token can call the Content Delivery API (the GraphQL endpoint at `https://graphql.datocms.com/`) to fetch published content. UI: Permissions > \"Access the Content Delivery API\" toggle.",
			},
			"can_access_cda_preview": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Whether the token can call the Content Delivery API with the `X-Include-Drafts: true` header to fetch draft (unpublished) content. UI: Permissions > \"Access the Content Delivery API in Preview Mode\" toggle.",
			},
			"can_access_cma": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Whether the token can access the Content Management API. The UI warns: depending on the role's permissions, enabling this option may allow write or administrative operations, so use with caution. UI: Permissions > \"Access the Content Management API\" toggle.",
			},
			"token": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "The token secret, used as the `Authorization: Bearer <token>` credential. Returned by the API on create and on reads whose caller role has `can_manage_access_tokens`; when the API returns null the value stored in the state is preserved. Rotation (POST `/access_tokens/{id}/regenerate_token`) is not managed by this resource: after an out-of-band rotation, refresh the state to pick up the new secret.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *AccessTokenResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// accessTokenAttributesFromModel converts the Terraform plan/state model into
// the JSON:API attribute set plus the role ID (always non-nil: role_id is a
// required attribute because the CMA rejects null role linkages).
func accessTokenAttributesFromModel(data *AccessTokenResourceModel) (accessTokenAttributes, *string) {
	attrs := accessTokenAttributes{
		Name:                data.Name.ValueString(),
		CanAccessCda:        data.CanAccessCda.ValueBool(),
		CanAccessCdaPreview: data.CanAccessCdaPreview.ValueBool(),
		CanAccessCma:        data.CanAccessCma.ValueBool(),
	}
	return attrs, stringPtr(data.RoleID)
}

// modelFromAccessToken maps an access token API response onto the resource
// model. The stored token secret is preserved when the API returns null for
// it (the API masks it for callers whose role lacks can_manage_access_tokens),
// the same drift protection used for the webhook basic auth password.
func modelFromAccessToken(accessToken *accessTokenData, data *AccessTokenResourceModel) {
	data.ID = types.StringValue(accessToken.ID)
	a := accessToken.Attributes
	data.Name = types.StringValue(a.Name)
	data.CanAccessCda = types.BoolValue(a.CanAccessCda)
	data.CanAccessCdaPreview = types.BoolValue(a.CanAccessCdaPreview)
	data.CanAccessCma = types.BoolValue(a.CanAccessCma)

	if a.Token != nil || data.Token.IsUnknown() {
		data.Token = stringFromPtr(a.Token)
	}

	if accessToken.Relationships != nil && accessToken.Relationships.Role.Data != nil {
		data.RoleID = types.StringValue(accessToken.Relationships.Role.Data.ID)
	} else {
		data.RoleID = types.StringNull()
	}
}

// --- CRUD ---

func (r *AccessTokenResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AccessTokenResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	attrs, roleID := accessTokenAttributesFromModel(&data)

	accessToken, err := r.client.CreateAccessToken(ctx, attrs, roleID)
	if err != nil {
		resp.Diagnostics.AddError("Error creating DatoCMS access token", err.Error())
		return
	}

	// The configured attributes are concrete in the plan (defaults cover the
	// computed flags), so only the server-assigned ID and the generated token
	// secret are taken from the response.
	data.ID = types.StringValue(accessToken.ID)
	data.Token = stringFromPtr(accessToken.Attributes.Token)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AccessTokenResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AccessTokenResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	accessToken, err := r.client.GetAccessToken(ctx, data.ID.ValueString())
	if err != nil {
		if errors.Is(err, errNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading DatoCMS access token", err.Error())
		return
	}

	modelFromAccessToken(accessToken, &data)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AccessTokenResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AccessTokenResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	attrs, roleID := accessTokenAttributesFromModel(&data)

	if _, err := r.client.UpdateAccessToken(ctx, data.ID.ValueString(), attrs, roleID); err != nil {
		resp.Diagnostics.AddError("Error updating DatoCMS access token", err.Error())
		return
	}

	// Updates never change the token secret (rotation is a separate endpoint),
	// so the planned value (carried over from state) is stored as-is.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AccessTokenResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AccessTokenResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteAccessToken(ctx, data.ID.ValueString()); err != nil {
		if errors.Is(err, errNotFound) {
			return
		}
		resp.Diagnostics.AddError("Error deleting DatoCMS access token", err.Error())
	}
}

func (r *AccessTokenResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
