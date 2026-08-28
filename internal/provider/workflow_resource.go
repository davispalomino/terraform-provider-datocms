// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &WorkflowResource{}
var _ resource.ResourceWithConfigure = &WorkflowResource{}
var _ resource.ResourceWithImportState = &WorkflowResource{}

func NewWorkflowResource() resource.Resource {
	return &WorkflowResource{}
}

// WorkflowResource defines the datocms_workflow resource implementation.
type WorkflowResource struct {
	client *DatoCMSClient
}

// WorkflowResourceModel describes the resource data model.
type WorkflowResourceModel struct {
	ID      types.String `tfsdk:"id"`
	Project types.String `tfsdk:"project"`
	Name    types.String `tfsdk:"name"`
	APIKey  types.String `tfsdk:"api_key"`
	Stages  types.List   `tfsdk:"stages"`
}

// workflowStageModel mirrors one element of stages.
type workflowStageModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Initial     types.Bool   `tfsdk:"initial"`
}

var workflowStageObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"id":          types.StringType,
	"name":        types.StringType,
	"description": types.StringType,
	"initial":     types.BoolType,
}}

func (r *WorkflowResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workflow"
}

func (r *WorkflowResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a DatoCMS workflow (UI: Settings > Workflows > \"New workflow\"). A workflow implements a state machine that lets content move through a series of custom approval stages, from draft to publication. Each stage is one entry of `stages`; the stage with `initial = true` is where new records enter the workflow.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Workflow identifier.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project": projectAttribute(),
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the workflow. UI: \"Name\".",
			},
			"api_key": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Workflow API key: the machine-readable identifier used to reference the workflow through the API (for example from role item type permissions). UI: \"API identifier\".",
			},
			"stages": schema.ListNestedAttribute{
				Required:            true,
				MarkdownDescription: "The ordered stages of the workflow (at least one). UI: \"Workflow stages\" > \"Add new stage\".",
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Machine-readable identifier of the stage (for example `waiting_for_review`).",
						},
						"name": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Human-readable name of the stage.",
						},
						"description": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "Description of the stage's purpose.",
						},
						"initial": schema.BoolAttribute{
							Optional:            true,
							Computed:            true,
							Default:             booldefault.StaticBool(false),
							MarkdownDescription: "Whether this is the initial stage of the workflow. Set it to `true` on exactly one stage.",
						},
					},
				},
			},
		},
	}
}

func (r *WorkflowResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// workflowAttributesFromModel converts the Terraform plan/state model into
// the full JSON:API attribute set. An omitted stage description maps to a nil
// pointer, which omits the field from the request (it is optional, not
// create-required).
func workflowAttributesFromModel(ctx context.Context, data *WorkflowResourceModel) (workflowAttributes, diag.Diagnostics) {
	var diags diag.Diagnostics

	attrs := workflowAttributes{
		Name:   data.Name.ValueString(),
		APIKey: data.APIKey.ValueString(),
	}

	var stageModels []workflowStageModel
	diags.Append(data.Stages.ElementsAs(ctx, &stageModels, false)...)
	if diags.HasError() {
		return attrs, diags
	}

	attrs.Stages = make([]workflowStage, 0, len(stageModels))
	for _, sm := range stageModels {
		attrs.Stages = append(attrs.Stages, workflowStage{
			ID:          sm.ID.ValueString(),
			Name:        sm.Name.ValueString(),
			Description: stringPtr(sm.Description),
			Initial:     sm.Initial.ValueBool(),
		})
	}

	return attrs, diags
}

// modelFromWorkflow maps a workflow API response onto the resource model. A
// null or missing stage description maps to null, matching an omitted
// optional attribute in the configuration.
func modelFromWorkflow(ctx context.Context, workflow *workflowData, data *WorkflowResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	var d diag.Diagnostics

	data.ID = types.StringValue(workflow.ID)
	a := workflow.Attributes
	data.Name = types.StringValue(a.Name)
	data.APIKey = types.StringValue(a.APIKey)

	stageModels := make([]workflowStageModel, 0, len(a.Stages))
	for _, stage := range a.Stages {
		stageModels = append(stageModels, workflowStageModel{
			ID:          types.StringValue(stage.ID),
			Name:        types.StringValue(stage.Name),
			Description: stringFromPtr(stage.Description),
			Initial:     types.BoolValue(stage.Initial),
		})
	}
	data.Stages, d = types.ListValueFrom(ctx, workflowStageObjectType, stageModels)
	diags.Append(d...)

	return diags
}

// --- CRUD ---

func (r *WorkflowResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data WorkflowResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	attrs, diags := workflowAttributesFromModel(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	client := clientForProject(r.client, data.Project, &resp.Diagnostics)
	if client == nil {
		return
	}

	workflow, err := client.CreateWorkflow(ctx, attrs)
	if err != nil {
		resp.Diagnostics.AddError("Error creating DatoCMS workflow", err.Error())
		return
	}

	// All attributes are concrete in the plan (the initial default covers the
	// computed one), so only the server-assigned ID is taken from the
	// response.
	data.ID = types.StringValue(workflow.ID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WorkflowResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data WorkflowResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client := clientForProject(r.client, data.Project, &resp.Diagnostics)
	if client == nil {
		return
	}

	workflow, err := client.GetWorkflow(ctx, data.ID.ValueString())
	if err != nil {
		if errors.Is(err, errNotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading DatoCMS workflow", err.Error())
		return
	}

	resp.Diagnostics.Append(modelFromWorkflow(ctx, workflow, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WorkflowResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data WorkflowResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	attrs, diags := workflowAttributesFromModel(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	client := clientForProject(r.client, data.Project, &resp.Diagnostics)
	if client == nil {
		return
	}

	if _, err := client.UpdateWorkflow(ctx, data.ID.ValueString(), attrs); err != nil {
		resp.Diagnostics.AddError("Error updating DatoCMS workflow", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *WorkflowResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data WorkflowResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	client := clientForProject(r.client, data.Project, &resp.Diagnostics)
	if client == nil {
		return
	}

	if err := client.DeleteWorkflow(ctx, data.ID.ValueString()); err != nil {
		if errors.Is(err, errNotFound) {
			return
		}
		resp.Diagnostics.AddError("Error deleting DatoCMS workflow", err.Error())
	}
}

func (r *WorkflowResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	importStateWithProject(ctx, req, resp)
}
