package provider

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"github.com/google/jsonapi"
	"io"
	"net/http"
	"strings"
	"terraform-provider-terrakube/internal/client"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &ModuleResource{}
var _ resource.ResourceWithImportState = &ModuleResource{}

type ModuleResource struct {
	client   *http.Client
	endpoint string
	token    string
}

type ModuleResourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	OrganizationId types.String `tfsdk:"organization_id"`
	Description    types.String `tfsdk:"description"`
	ProviderName   types.String `tfsdk:"provider_name"`
	Source         types.String `tfsdk:"source"`
}

func NewModuleResource() resource.Resource {
	return &ModuleResource{}
}

func (r *ModuleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_module"
}

func (r *ModuleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Module Id",
			},
			"organization_id": schema.StringAttribute{
				Required:    true,
				Description: "Terrakube organization id",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Module name",
			},
			"description": schema.StringAttribute{
				Required:    true,
				Description: "Module description",
			},
			"provider_name": schema.StringAttribute{
				Required:    true,
				Description: "Module provider name. Example: azurerm, google, aws, etc",
			},
			"source": schema.StringAttribute{
				Required:    true,
				Description: "Source (git using https or ssh protocol)",
			},
		},
	}
}

func (r *ModuleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	providerData, ok := req.ProviderData.(*TerrakubeConnectionData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Module Resource Configure Type",
			fmt.Sprintf("Expected *TerrakubeConnectionData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	customTransport := http.DefaultTransport.(*http.Transport).Clone()
	customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	client := http.Client{Transport: customTransport}

	r.client = &client
	r.endpoint = providerData.Endpoint
	r.token = providerData.Token

	tflog.Debug(ctx, "Configuring Module resource", map[string]any{"success": true})
}

func (r *ModuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ModuleResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	bodyRequest := &client.ModuleEntity{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
		Provider:    plan.ProviderName.ValueString(),
		Source:      plan.Source.ValueString(),
	}

	var out = new(bytes.Buffer)
	jsonapi.MarshalPayload(out, bodyRequest)

	moduleRequest, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/organization/%s/module", r.endpoint, plan.OrganizationId.ValueString()), strings.NewReader(out.String()))
	moduleRequest.Header.Add("Authorization", fmt.Sprintf("Bearer %s", r.token))
	moduleRequest.Header.Add("Content-Type", "application/vnd.api+json")
	if err != nil {
		resp.Diagnostics.AddError("Error creating module resource request", fmt.Sprintf("Error creating team resource request: %s", err))
		return
	}

	moduleResponse, err := r.client.Do(moduleRequest)
	if err != nil {
		resp.Diagnostics.AddError("Error executing module resource request", fmt.Sprintf("Error executing team resource request: %s", err))
		return
	}

	bodyResponse, err := io.ReadAll(moduleResponse.Body)
	if err != nil {
		tflog.Error(ctx, "Error reading module resource response")
	}

	tflog.Info(ctx, "Body Response", map[string]any{"bodyResponse": string(bodyResponse)})

	newModule := &client.ModuleEntity{}

	err = jsonapi.UnmarshalPayload(strings.NewReader(string(bodyResponse)), newModule)

	if err != nil {
		resp.Diagnostics.AddError("Error unmarshal payload response", fmt.Sprintf("Error unmarshal payload response: %s", err))
		return
	}

	tflog.Info(ctx, "Body Response", map[string]any{"bodyResponse": string(bodyResponse)})

	plan.ID = types.StringValue(newModule.ID)
	plan.Name = types.StringValue(newModule.Name)
	plan.Description = types.StringValue(newModule.Description)
	plan.ProviderName = types.StringValue(newModule.Provider)
	plan.Source = types.StringValue(newModule.Source)

	tflog.Info(ctx, "Module Resource Created", map[string]any{"success": true})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ModuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ModuleResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	moduleRequest, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/v1/organization/%s/module/%s", r.endpoint, state.OrganizationId.ValueString(), state.ID.ValueString()), nil)
	moduleRequest.Header.Add("Authorization", fmt.Sprintf("Bearer %s", r.token))
	moduleRequest.Header.Add("Content-Type", "application/vnd.api+json")
	if err != nil {
		resp.Diagnostics.AddError("Error creating module resource request", fmt.Sprintf("Error creating team resource request: %s", err))
		return
	}

	moduleResponse, err := r.client.Do(moduleRequest)
	if err != nil {
		resp.Diagnostics.AddError("Error executing module resource request", fmt.Sprintf("Error executing team resource request: %s", err))
		return
	}

	bodyResponse, err := io.ReadAll(moduleResponse.Body)
	if err != nil {
		tflog.Error(ctx, "Error reading module resource response")
	}
	module := &client.ModuleEntity{}

	tflog.Info(ctx, "Body Response", map[string]any{"bodyResponse": string(bodyResponse)})
	err = jsonapi.UnmarshalPayload(strings.NewReader(string(bodyResponse)), module)

	if err != nil {
		resp.Diagnostics.AddError("Error unmarshal payload response", fmt.Sprintf("Error unmarshal payload response: %s", err))
		return
	}

	tflog.Info(ctx, "Body Response", map[string]any{"bodyResponse": string(bodyResponse)})

	state.Name = types.StringValue(module.Name)
	state.Description = types.StringValue(module.Description)
	state.ProviderName = types.StringValue(module.Provider)
	state.Source = types.StringValue(module.Source)

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Module Resource reading", map[string]any{"success": true})
}

func (r *ModuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan ModuleResourceModel
	var state ModuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bodyRequest := &client.ModuleEntity{
		ID:          state.ID.ValueString(),
		Name:        plan.Name.ValueString(),
		Description: plan.Name.ValueString(),
		Provider:    plan.ProviderName.ValueString(),
		Source:      plan.Source.ValueString(),
	}

	var out = new(bytes.Buffer)
	jsonapi.MarshalPayload(out, bodyRequest)

	moduleRequest, err := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/api/v1/organization/%s/module/%s", r.endpoint, state.OrganizationId.ValueString(), state.ID.ValueString()), strings.NewReader(out.String()))
	moduleRequest.Header.Add("Authorization", fmt.Sprintf("Bearer %s", r.token))
	moduleRequest.Header.Add("Content-Type", "application/vnd.api+json")
	if err != nil {
		resp.Diagnostics.AddError("Error creating module resource request", fmt.Sprintf("Error creating team resource request: %s", err))
		return
	}

	teamResponse, err := r.client.Do(moduleRequest)
	if err != nil {
		resp.Diagnostics.AddError("Error executing module resource request", fmt.Sprintf("Error executing team resource request: %s", err))
		return
	}

	bodyResponse, err := io.ReadAll(teamResponse.Body)
	if err != nil {
		tflog.Error(ctx, "Error reading module resource response")
	}

	tflog.Info(ctx, "Body Response", map[string]any{"success": string(bodyResponse)})

	moduleRequest, err = http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/v1/organization/%s/module/%s", r.endpoint, state.OrganizationId.ValueString(), state.ID.ValueString()), nil)
	moduleRequest.Header.Add("Authorization", fmt.Sprintf("Bearer %s", r.token))
	moduleRequest.Header.Add("Content-Type", "application/vnd.api+json")
	if err != nil {
		resp.Diagnostics.AddError("Error creating module resource request", fmt.Sprintf("Error creating team resource request: %s", err))
		return
	}

	teamResponse, err = r.client.Do(moduleRequest)
	if err != nil {
		resp.Diagnostics.AddError("Error executing module resource request", fmt.Sprintf("Error executing team resource request: %s", err))
		return
	}

	bodyResponse, err = io.ReadAll(teamResponse.Body)
	if err != nil {
		resp.Diagnostics.AddError("Error reading module resource response body", fmt.Sprintf("Error reading team resource response body: %s", err))
	}

	tflog.Info(ctx, "Body Response", map[string]any{"bodyResponse": string(bodyResponse)})

	module := &client.ModuleEntity{}
	err = jsonapi.UnmarshalPayload(strings.NewReader(string(bodyResponse)), module)

	if err != nil {
		resp.Diagnostics.AddError("Error unmarshal payload response", fmt.Sprintf("Error unmarshal payload response: %s", err))
		return
	}

	plan.ID = types.StringValue(state.ID.ValueString())
	plan.Name = types.StringValue(module.Name)
	plan.Description = types.StringValue(module.Description)
	plan.ProviderName = types.StringValue(module.Provider)
	plan.Source = types.StringValue(module.Source)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ModuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ModuleResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	reqOrg, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v1/organization/%s/module/%s", r.endpoint, data.OrganizationId.ValueString(), data.ID.ValueString()), nil)
	reqOrg.Header.Add("Authorization", fmt.Sprintf("Bearer %s", r.token))
	if err != nil {
		resp.Diagnostics.AddError("Error creating module resource request", fmt.Sprintf("Error creating team resource request: %s", err))
		return
	}

	_, err = r.client.Do(reqOrg)
	if err != nil {
		resp.Diagnostics.AddError("Error executing module resource request", fmt.Sprintf("Error executing team resource request: %s", err))
		return
	}
}

func (r *ModuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
