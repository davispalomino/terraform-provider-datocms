// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// projectAttribute returns the shared schema of the optional project
// attribute, which selects the API token used for the resource among the
// entries of the provider's api_tokens map.
func projectAttribute() schema.StringAttribute {
	return schema.StringAttribute{
		Optional:            true,
		MarkdownDescription: "Key of the provider's `api_tokens` map whose token is used for all API calls of this resource. When omitted, the default token (`api_token` attribute or `DATOCMS_API_TOKEN` environment variable) is used. Changing this attribute forces the resource to be recreated, since each key targets a different DatoCMS project.",
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.RequiresReplace(),
		},
	}
}

// clientForProject resolves the client for the resource's project attribute,
// converting resolution failures into a diagnostic. Returns nil when a
// diagnostic was added.
func clientForProject(client *DatoCMSClient, project types.String, diags *diag.Diagnostics) *DatoCMSClient {
	scoped, err := client.forProject(project.ValueString())
	if err != nil {
		diags.AddAttributeError(
			path.Root("project"),
			"Unknown DatoCMS project",
			err.Error(),
		)
		return nil
	}
	return scoped
}

// importStateWithProject imports a resource from either a plain resource ID
// (default token) or a compound "project/id" import ID, storing both the id
// and project attributes.
func importStateWithProject(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	project, id := parseImportID(req.ID)
	if id == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			"Expected a resource ID or a compound \"project/id\" import ID, got: "+req.ID,
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), id)...)
	if project != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project"), project)...)
	}
}
