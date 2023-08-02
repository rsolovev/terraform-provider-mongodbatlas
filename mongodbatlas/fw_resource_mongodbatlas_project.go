package mongodbatlas

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"time"

	"go.mongodb.org/atlas-sdk/v20230201002/admin"
	matlas "go.mongodb.org/atlas/mongodbatlas"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
)

var _ resource.Resource = &ProjectResource{}
var _ resource.ResourceWithImportState = &ProjectResource{}

func NewProjectResource() resource.Resource {
	return &ProjectResource{}
}

type ProjectResource struct {
	client *MongoDBClient
}

type tfProjectResourceModel struct {
	ID                                          types.String `tfsdk:"id"`
	Name                                        types.String `tfsdk:"name"`
	OrgID                                       types.String `tfsdk:"org_id"`
	ClusterCount                                types.Int64  `tfsdk:"cluster_count"`
	Created                                     types.String `tfsdk:"created"`
	ProjectOwnerID                              types.String `tfsdk:"project_owner_id"`
	WithDefaultAlertsSettings                   types.Bool   `tfsdk:"with_default_alerts_settings"`
	IsCollectDatabaseSpecificsStatisticsEnabled types.Bool   `tfsdk:"is_collect_database_specifics_statistics_enabled"`
	IsDataExplorerEnabled                       types.Bool   `tfsdk:"is_data_explorer_enabled"`
	IsExtendedStorageSizesEnabled               types.Bool   `tfsdk:"is_extended_storage_sizes_enabled"`
	IsPerformanceAdvisorEnabled                 types.Bool   `tfsdk:"is_performance_advisor_enabled"`
	IsRealtimePerformancePanelEnabled           types.Bool   `tfsdk:"is_realtime_performance_panel_enabled"`
	IsSchemaAdvisorEnabled                      types.Bool   `tfsdk:"is_schema_advisor_enabled"`
	RegionUsageRestrictions                     types.String `tfsdk:"region_usage_restrictions"`
	Teams                                       types.Set    `tfsdk:"teams"`
	Limits                                      types.Set    `tfsdk:"limits"`
}

type tfTeamModel struct {
	TeamID    types.String `tfsdk:"team_id"`
	RoleNames types.Set    `tfsdk:"role_names"`
}

type tfLimitModel struct {
	Name         types.String `tfsdk:"name"`
	Value        types.Int64  `tfsdk:"value"`
	CurrentUsage types.Int64  `tfsdk:"current_usage"`
	DefaultLimit types.Int64  `tfsdk:"default_limit"`
	MaximumLimit types.Int64  `tfsdk:"maximum_limit"`
}

var tfTeamObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"team_id":    types.StringType,
	"role_names": types.SetType{ElemType: types.StringType},
}}
var tfLimitObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"name":          types.StringType,
	"value":         types.Int64Type,
	"current_usage": types.Int64Type,
	"default_limit": types.Int64Type,
	"maximum_limit": types.Int64Type,
}}

func (r *ProjectResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (r *ProjectResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"org_id": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"cluster_count": schema.Int64Attribute{
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"created": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_owner_id": schema.StringAttribute{
				Optional: true,
			},
			"with_default_alerts_settings": schema.BoolAttribute{
				// This needs to be Computed now otherwise Terraform throws error:
				// Schema Using Attribute Default For Non-Computed Attribute
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"is_collect_database_specifics_statistics_enabled": schema.BoolAttribute{
				Computed: true,
				Optional: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"is_data_explorer_enabled": schema.BoolAttribute{
				Computed: true,
				Optional: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"is_extended_storage_sizes_enabled": schema.BoolAttribute{
				Computed: true,
				Optional: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"is_performance_advisor_enabled": schema.BoolAttribute{
				Computed: true,
				Optional: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"is_realtime_performance_panel_enabled": schema.BoolAttribute{
				Computed: true,
				Optional: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"is_schema_advisor_enabled": schema.BoolAttribute{
				Computed: true,
				Optional: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"region_usage_restrictions": schema.StringAttribute{
				Optional: true,
				// This is only set during Create in SDKv2 resource, this should not be computed
				// Computed: true,
			},
		},
		Blocks: map[string]schema.Block{
			"teams": schema.SetNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"team_id": schema.StringAttribute{
							Required: true,
						},
						"role_names": schema.SetAttribute{
							Required:    true,
							ElementType: types.StringType,
						},
					},
				},
			},
			"limits": schema.SetNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required: true,
						},
						"value": schema.Int64Attribute{
							Required: true,
						},
						"current_usage": schema.Int64Attribute{
							Computed: true,
						},
						"default_limit": schema.Int64Attribute{
							Computed: true,
						},
						"maximum_limit": schema.Int64Attribute{
							Computed: true,
						},
					},
				},
				// https://discuss.hashicorp.com/t/computed-attributes-and-plan-modifiers/45830/12
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *ProjectResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*MongoDBClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *MongoDBClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *ProjectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var projectPlan tfProjectResourceModel
	var teams []tfTeamModel
	var limits []tfLimitModel

	conn := r.client.Atlas
	connV2 := r.client.AtlasV2

	diags := req.Plan.Get(ctx, &projectPlan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectReq := &matlas.Project{
		OrgID:                     projectPlan.OrgID.ValueString(),
		Name:                      projectPlan.Name.ValueString(),
		WithDefaultAlertsSettings: projectPlan.WithDefaultAlertsSettings.ValueBoolPointer(),
		RegionUsageRestrictions:   projectPlan.RegionUsageRestrictions.ValueString(),
	}

	var createProjectOptions *matlas.CreateProjectOptions

	if !projectPlan.ProjectOwnerID.IsNull() {
		createProjectOptions = &matlas.CreateProjectOptions{
			ProjectOwnerID: projectPlan.ProjectOwnerID.ValueString(),
		}
	}

	// CREATE PROJECT
	project, _, err := conn.Projects.Create(ctx, projectReq, createProjectOptions)
	if err != nil {
		resp.Diagnostics.AddError(errorProjectCreate, err.Error())
		return
	}

	// ADD TEAMS
	if len(projectPlan.Teams.Elements()) > 0 {
		_ = projectPlan.Teams.ElementsAs(ctx, &teams, false)

		_, _, err := conn.Projects.AddTeamsToProject(ctx, project.ID, toAtlasProjectTeams(ctx, teams))
		if err != nil {
			errd := deleteProject(ctx, conn, project.ID)
			if errd != nil {
				resp.Diagnostics.AddError("error during project deletion when adding teams", fmt.Sprintf(errorProjectDelete, project.ID, err.Error()))
				return
			}
			resp.Diagnostics.AddError("error adding teams into the project", err.Error())
			return
		}
	}

	// ADD LIMITS
	if len(projectPlan.Limits.Elements()) > 0 {
		_ = projectPlan.Limits.ElementsAs(ctx, &limits, false)

		for _, limit := range limits {
			dataFederationLimit := &admin.DataFederationLimit{
				Name:  limit.Name.ValueString(),
				Value: limit.Value.ValueInt64(),
			}
			_, _, err := connV2.ProjectsApi.SetProjectLimit(ctx, limit.Name.ValueString(), project.ID, dataFederationLimit).Execute()
			if err != nil {
				errd := deleteProject(ctx, conn, project.ID)
				if errd != nil {
					resp.Diagnostics.AddError("error during project deletion when adding limits", fmt.Sprintf(errorProjectDelete, project.ID, err.Error()))
					return
				}
				resp.Diagnostics.AddError("error adding limits into the project", err.Error())
				return
			}
		}
	}

	// ADD SETTINGS
	projectSettings, _, err := conn.Projects.GetProjectSettings(ctx, project.ID)
	if err != nil {
		errd := deleteProject(ctx, conn, project.ID)
		if errd != nil {
			resp.Diagnostics.AddError("error during project deletion when getting project settings", fmt.Sprintf(errorProjectDelete, project.ID, err.Error()))
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("error getting project's settings assigned (%s):", project.ID), err.Error())
		return
	}

	if !projectPlan.IsCollectDatabaseSpecificsStatisticsEnabled.IsUnknown() {
		projectSettings.IsCollectDatabaseSpecificsStatisticsEnabled = projectPlan.IsCollectDatabaseSpecificsStatisticsEnabled.ValueBoolPointer()
	}
	if !projectPlan.IsDataExplorerEnabled.IsUnknown() {
		projectSettings.IsDataExplorerEnabled = projectPlan.IsDataExplorerEnabled.ValueBoolPointer()
	}
	if !projectPlan.IsExtendedStorageSizesEnabled.IsUnknown() {
		projectSettings.IsExtendedStorageSizesEnabled = projectPlan.IsExtendedStorageSizesEnabled.ValueBoolPointer()
	}
	if !projectPlan.IsPerformanceAdvisorEnabled.IsUnknown() {
		projectSettings.IsPerformanceAdvisorEnabled = projectPlan.IsPerformanceAdvisorEnabled.ValueBoolPointer()
	}
	if !projectPlan.IsRealtimePerformancePanelEnabled.IsUnknown() {
		projectSettings.IsRealtimePerformancePanelEnabled = projectPlan.IsRealtimePerformancePanelEnabled.ValueBoolPointer()
	}
	if !projectPlan.IsSchemaAdvisorEnabled.IsUnknown() {
		projectSettings.IsSchemaAdvisorEnabled = projectPlan.IsSchemaAdvisorEnabled.ValueBoolPointer()
	}

	_, _, err = conn.Projects.UpdateProjectSettings(ctx, project.ID, projectSettings)
	if err != nil {
		errd := deleteProject(ctx, conn, project.ID)
		if errd != nil {
			resp.Diagnostics.AddError("error during project deletion when updating project settings", fmt.Sprintf(errorProjectDelete, project.ID, err.Error()))
			return
		}
		resp.Diagnostics.AddError(fmt.Sprintf("error updating project's settings assigned (%s):", project.ID), err.Error())
		return
	}

	// READ
	// GET PROJECT
	projectID := project.ID
	projectRes, atlasResp, err := conn.Projects.GetOneProject(ctx, projectID)
	if err != nil {
		if resp != nil && atlasResp.StatusCode == http.StatusNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("error when getting project after create", fmt.Sprintf(errorProjectRead, projectID, err.Error()))
		return
	}

	// GET PROJECT PROPS
	atlasteams, atlaslimits, atlasprojectSettings, err := getProjectPropsFromAPI(ctx, conn, connV2, projectID)
	if err != nil {
		resp.Diagnostics.AddError("error when getting project properties after create", fmt.Sprintf(errorProjectRead, projectID, err.Error()))
		return
	}

	atlaslimits = filterUserDefinedLimits(atlaslimits, limits)
	projectPlanNew := toTFProjectResourceModel(ctx, projectRes, atlasteams, atlasprojectSettings, atlaslimits)
	updatePlanFromConfig2(projectPlanNew, projectPlan)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, projectPlanNew)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *ProjectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var projectState tfProjectResourceModel
	var limits []tfLimitModel
	conn := r.client.Atlas
	connV2 := r.client.AtlasV2

	// Get current state
	diags := req.State.Get(ctx, &projectState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := projectState.ID.ValueString()
	if len(projectState.Limits.Elements()) > 0 {
		_ = projectState.Limits.ElementsAs(ctx, &limits, false)
	}

	// GET PROJECT
	projectRes, atlasResp, err := conn.Projects.GetOneProject(ctx, projectID)
	if err != nil {
		if resp != nil && atlasResp.StatusCode == http.StatusNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("error when getting project from Atlas", fmt.Sprintf(errorProjectRead, projectID, err.Error()))
		return
	}

	// GET PROJECT PROPS
	atlasteams, atlaslimits, atlasprojectSettings, err := getProjectPropsFromAPI(ctx, conn, connV2, projectID)
	if err != nil {
		resp.Diagnostics.AddError("error when getting project properties after create", fmt.Sprintf(errorProjectRead, projectID, err.Error()))
		return
	}

	atlaslimits = filterUserDefinedLimits(atlaslimits, limits)
	projectStateNew := toTFProjectResourceModel(ctx, projectRes, atlasteams, atlasprojectSettings, atlaslimits)
	updatePlanFromConfig2(projectStateNew, projectState)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &projectStateNew)...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r *ProjectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var projectState tfProjectResourceModel
	var projectPlan tfProjectResourceModel
	conn := r.client.Atlas
	connV2 := r.client.AtlasV2

	// Get current state
	resp.Diagnostics.Append(req.State.Get(ctx, &projectState)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Get current plan
	resp.Diagnostics.Append(req.Plan.Get(ctx, &projectPlan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	projectID := projectState.ID.ValueString()

	err := updateProject2(ctx, conn, projectState, projectPlan)
	if err != nil {
		resp.Diagnostics.AddError("error in project update", fmt.Sprintf(errorProjectUpdate, projectID, err.Error()))
		return
	}

	err = updateProjectTeams(ctx, conn, projectState, projectPlan)
	if err != nil {
		resp.Diagnostics.AddError("error in project teams update", fmt.Sprintf(errorProjectUpdate, projectID, err.Error()))
		return
	}

	err = updateProjectLimits(ctx, connV2, projectState, projectPlan)
	if err != nil {
		resp.Diagnostics.AddError("error in project limits update", fmt.Sprintf(errorProjectUpdate, projectID, err.Error()))
		return
	}

	err = updateProjectSettings(ctx, conn, projectState, projectPlan)
	if err != nil {
		resp.Diagnostics.AddError("error in project settings update", fmt.Sprintf(errorProjectUpdate, projectID, err.Error()))
		return
	}

	// READ
	// GET PROJECT
	projectRes, atlasResp, err := conn.Projects.GetOneProject(ctx, projectID)
	if err != nil {
		if resp != nil && atlasResp.StatusCode == http.StatusNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("error when getting project after create", fmt.Sprintf(errorProjectRead, projectID, err.Error()))
		return
	}

	// GET PROJECT PROPS
	atlasteams, atlaslimits, atlasprojectSettings, err := getProjectPropsFromAPI(ctx, conn, connV2, projectID)
	if err != nil {
		resp.Diagnostics.AddError("error when getting project properties after create", fmt.Sprintf(errorProjectRead, projectID, err.Error()))
		return
	}
	var planLimits []tfLimitModel
	_ = projectPlan.Limits.ElementsAs(ctx, &planLimits, false)
	atlaslimits = filterUserDefinedLimits(atlaslimits, planLimits)
	projectPlanNew := toTFProjectResourceModel(ctx, projectRes, atlasteams, atlasprojectSettings, atlaslimits)
	updatePlanFromConfig2(projectPlanNew, projectPlan)

	// Save updated data into Terraform state.
	resp.Diagnostics.Append(resp.State.Set(ctx, &projectPlanNew)...)
}

func (r *ProjectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var project *tfProjectResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &project)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := project.ID.ValueString()
	err := deleteProject2(ctx, r.client.Atlas, projectID)

	if err != nil {
		resp.Diagnostics.AddError("error when destroying resource", fmt.Sprintf(errorProjectDelete, projectID, err.Error()))
		return
	}
}

func (r *ProjectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func updatePlanFromConfig2(projectPlanNewPtr *tfProjectResourceModel, projectPlan tfProjectResourceModel) {
	// we need to reset defaults from what was previously in the state:
	// https://discuss.hashicorp.com/t/boolean-optional-default-value-migration-to-framework/55932
	projectPlanNewPtr.WithDefaultAlertsSettings = projectPlan.WithDefaultAlertsSettings
	projectPlanNewPtr.ProjectOwnerID = projectPlan.ProjectOwnerID
}

func filterUserDefinedLimits(fetchedLimits []admin.DataFederationLimit, tflimits []tfLimitModel) []admin.DataFederationLimit {
	definedLimitsMap := make(map[string]tfLimitModel)
	for _, definedLimit := range tflimits {
		definedLimitsMap[definedLimit.Name.ValueString()] = definedLimit
	}

	filteredLimits := []admin.DataFederationLimit{}
	for i := range fetchedLimits {
		limitRes := fetchedLimits[i]
		if _, ok := definedLimitsMap[limitRes.Name]; ok {
			filteredLimits = append(filteredLimits, limitRes)
		}
	}
	return filteredLimits
}

func getProjectPropsFromAPI(ctx context.Context, conn *matlas.Client, connV2 *admin.APIClient, projectID string) (*matlas.TeamsAssigned, []admin.DataFederationLimit, *matlas.ProjectSettings, error) {
	teams, _, err := conn.Projects.GetProjectTeamsAssigned(ctx, projectID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error getting project's teams assigned (%s): %v", projectID, err.Error())
	}

	limits, _, err := connV2.ProjectsApi.ListProjectLimits(ctx, projectID).Execute()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error getting project's limits (%s): %s", projectID, err.Error())
	}

	projectSettings, _, err := conn.Projects.GetProjectSettings(ctx, projectID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error getting project's settings assigned (%s): %v", projectID, err.Error())
	}

	return teams, limits, projectSettings, nil
}

func toTFProjectResourceModel(ctx context.Context, projectRes *matlas.Project,
	teams *matlas.TeamsAssigned, projectSettings *matlas.ProjectSettings, limits []admin.DataFederationLimit) *tfProjectResourceModel {
	projectPlan := tfProjectResourceModel{
		ID:                        types.StringValue(projectRes.ID),
		Name:                      types.StringValue(projectRes.Name),
		OrgID:                     types.StringValue(projectRes.OrgID),
		ClusterCount:              types.Int64Value(int64(projectRes.ClusterCount)),
		Created:                   types.StringValue(projectRes.Created),
		WithDefaultAlertsSettings: types.BoolPointerValue(projectRes.WithDefaultAlertsSettings),
		IsCollectDatabaseSpecificsStatisticsEnabled: types.BoolValue(*projectSettings.IsCollectDatabaseSpecificsStatisticsEnabled),
		IsDataExplorerEnabled:                       types.BoolValue(*projectSettings.IsDataExplorerEnabled),
		IsExtendedStorageSizesEnabled:               types.BoolValue(*projectSettings.IsExtendedStorageSizesEnabled),
		IsPerformanceAdvisorEnabled:                 types.BoolValue(*projectSettings.IsPerformanceAdvisorEnabled),
		IsRealtimePerformancePanelEnabled:           types.BoolValue(*projectSettings.IsRealtimePerformancePanelEnabled),
		IsSchemaAdvisorEnabled:                      types.BoolValue(*projectSettings.IsSchemaAdvisorEnabled),
		Teams:                                       toTFTeamsResourceModel(ctx, teams),
		Limits:                                      toTFLimitsResourceModel(ctx, limits),
	}

	return &projectPlan
}

func toTFLimitsResourceModel(ctx context.Context, dataFederationLimits []admin.DataFederationLimit) types.Set {
	limits := make([]tfLimitModel, len(dataFederationLimits))

	for i, dataFederationLimit := range dataFederationLimits {
		limits[i] = tfLimitModel{
			Name:         types.StringValue(dataFederationLimit.Name),
			Value:        types.Int64Value(dataFederationLimit.Value),
			CurrentUsage: types.Int64PointerValue(dataFederationLimit.CurrentUsage),
			DefaultLimit: types.Int64PointerValue(dataFederationLimit.DefaultLimit),
			MaximumLimit: types.Int64PointerValue(dataFederationLimit.MaximumLimit),
		}
	}

	s, _ := types.SetValueFrom(ctx, tfLimitObjectType, limits)
	return s
}

func toTFTeamsResourceModel(ctx context.Context, atlasTeams *matlas.TeamsAssigned) types.Set {
	teams := make([]tfTeamModel, atlasTeams.TotalCount)

	for i, atlasTeam := range atlasTeams.Results {
		roleNames, _ := types.SetValueFrom(ctx, types.StringType, atlasTeam.RoleNames)

		teams[i] = tfTeamModel{
			TeamID:    types.StringValue(atlasTeam.TeamID),
			RoleNames: roleNames,
		}
	}

	s, _ := types.SetValueFrom(ctx, tfTeamObjectType, teams)
	return s
}

func toAtlasProjectTeams(ctx context.Context, teams []tfTeamModel) []*matlas.ProjectTeam {
	res := make([]*matlas.ProjectTeam, len(teams))

	for i, team := range teams {
		res[i] = &matlas.ProjectTeam{
			TeamID:    team.TeamID.ValueString(),
			RoleNames: TypesSetToString(ctx, team.RoleNames),
		}
	}
	return res
}

func updateProjectSettings(ctx context.Context, conn *matlas.Client, projectState, projectPlan tfProjectResourceModel) error {
	hasChanged := false
	projectID := projectState.ID.ValueString()
	projectSettings, _, err := conn.Projects.GetProjectSettings(ctx, projectID)
	if err != nil {
		return fmt.Errorf("error getting project's settings assigned: %v", err.Error())
	}

	if projectPlan.IsCollectDatabaseSpecificsStatisticsEnabled != projectState.IsCollectDatabaseSpecificsStatisticsEnabled {
		hasChanged = true
		projectSettings.IsCollectDatabaseSpecificsStatisticsEnabled = projectPlan.IsCollectDatabaseSpecificsStatisticsEnabled.ValueBoolPointer()
	}
	if projectPlan.IsDataExplorerEnabled != projectState.IsDataExplorerEnabled {
		hasChanged = true
		projectSettings.IsDataExplorerEnabled = projectPlan.IsDataExplorerEnabled.ValueBoolPointer()
	}
	if projectPlan.IsExtendedStorageSizesEnabled != projectState.IsExtendedStorageSizesEnabled {
		hasChanged = true
		projectSettings.IsExtendedStorageSizesEnabled = projectPlan.IsExtendedStorageSizesEnabled.ValueBoolPointer()
	}
	if projectPlan.IsPerformanceAdvisorEnabled != projectState.IsPerformanceAdvisorEnabled {
		hasChanged = true
		projectSettings.IsPerformanceAdvisorEnabled = projectPlan.IsPerformanceAdvisorEnabled.ValueBoolPointer()
	}
	if projectPlan.IsRealtimePerformancePanelEnabled != projectState.IsRealtimePerformancePanelEnabled {
		hasChanged = true
		projectSettings.IsRealtimePerformancePanelEnabled = projectPlan.IsRealtimePerformancePanelEnabled.ValueBoolPointer()
	}
	if projectPlan.IsSchemaAdvisorEnabled != projectState.IsSchemaAdvisorEnabled {
		hasChanged = true
		projectSettings.IsSchemaAdvisorEnabled = projectPlan.IsSchemaAdvisorEnabled.ValueBoolPointer()
	}

	if hasChanged {
		_, _, err = conn.Projects.UpdateProjectSettings(ctx, projectID, projectSettings)
		if err != nil {
			return fmt.Errorf("error updating project's settings assigned: %v", err.Error())
		}
	}
	return nil
}

func updateProjectLimits(ctx context.Context, connV2 *admin.APIClient, projectState, projectPlan tfProjectResourceModel) error {
	var planLimits []tfLimitModel
	var stateLimits []tfLimitModel
	_ = projectPlan.Limits.ElementsAs(ctx, &planLimits, false)
	_ = projectState.Limits.ElementsAs(ctx, &stateLimits, false)

	if !hasLimitsChanged(planLimits, stateLimits) {
		return nil
	}

	projectID := projectState.ID.ValueString()

	// Removing limits from the project
	for _, limit := range stateLimits {
		limitName := limit.Name.ValueString()
		_, _, err := connV2.ProjectsApi.DeleteProjectLimit(ctx, limitName, projectID).Execute()
		if err != nil {
			return fmt.Errorf("error removing limit %s from the project(%s) during update: %s", limitName, projectID, err)
		}
	}

	// adding updated limits into the project
	if len(planLimits) > 0 {
		err := setProjectLimits(ctx, connV2, projectID, planLimits)
		if err != nil {
			return fmt.Errorf("error adding limits into the project during update: %v", err.Error())
		}
	}
	return nil
}

func setProjectLimits(ctx context.Context, connV2 *admin.APIClient, projectID string, tfLimits []tfLimitModel) error {
	for _, limit := range tfLimits {
		dataFederationLimit := &admin.DataFederationLimit{
			Name:  limit.Name.ValueString(),
			Value: limit.Value.ValueInt64(),
		}
		_, _, err := connV2.ProjectsApi.SetProjectLimit(ctx, limit.Name.ValueString(), projectID, dataFederationLimit).Execute()
		if err != nil {
			return fmt.Errorf("error adding limits into the project: %v", err.Error())
		}
	}
	return nil
}

func updateProjectTeams(ctx context.Context, conn *matlas.Client, projectState, projectPlan tfProjectResourceModel) error {
	var planTeams []tfTeamModel
	var stateTeams []tfTeamModel
	_ = projectPlan.Teams.ElementsAs(ctx, &planTeams, false)
	_ = projectState.Teams.ElementsAs(ctx, &stateTeams, false)

	if !hasTeamsChanged(planTeams, stateTeams) {
		return nil
	}

	projectID := projectState.ID.ValueString()

	// remove all current teams
	for _, team := range stateTeams {
		_, err := conn.Teams.RemoveTeamFromProject(ctx, projectID, team.TeamID.ValueString())
		if err != nil {
			return fmt.Errorf("error removing team from the project: %v", err.Error())
		}
	}

	// adding updated teams into the project
	if len(planTeams) > 0 {
		_, _, err := conn.Projects.AddTeamsToProject(ctx, projectID, toAtlasProjectTeams(ctx, planTeams))
		if err != nil {
			return fmt.Errorf("error adding teams to the project: %v", err.Error())

		}
	}
	return nil
}

func hasTeamsChanged(planTeams, stateTeams []tfTeamModel) bool {
	sort.Slice(planTeams, func(i, j int) bool {
		return planTeams[i].TeamID.ValueString() < planTeams[j].TeamID.ValueString()
	})
	sort.Slice(stateTeams, func(i, j int) bool {
		return stateTeams[i].TeamID.ValueString() < stateTeams[j].TeamID.ValueString()
	})
	return !reflect.DeepEqual(planTeams, stateTeams)
}

func hasLimitsChanged(planLimits, stateLimits []tfLimitModel) bool {
	sort.Slice(planLimits, func(i, j int) bool {
		return planLimits[i].Name.ValueString() < planLimits[j].Name.ValueString()
	})
	sort.Slice(stateLimits, func(i, j int) bool {
		return stateLimits[i].Name.ValueString() < stateLimits[j].Name.ValueString()
	})
	return !reflect.DeepEqual(planLimits, stateLimits)
}

func updateProject2(ctx context.Context, conn *matlas.Client, projectState, projectPlan tfProjectResourceModel) error {
	if projectPlan.Name.Equal(projectState.Name) {
		return nil
	}

	projectID := projectState.ID.ValueString()

	if _, _, err := conn.Projects.Update(ctx, projectID, newProjectUpdateRequest2(projectPlan)); err != nil {
		return fmt.Errorf("error updating the project(%s): %s", projectID, err)
	}

	return nil
}

func newProjectUpdateRequest2(tfProject tfProjectResourceModel) *matlas.ProjectUpdateRequest {
	return &matlas.ProjectUpdateRequest{
		Name: tfProject.Name.ValueString(),
	}
}

func deleteProject2(ctx context.Context, conn *matlas.Client, projectID string) error {
	stateConf := &retry.StateChangeConf{
		Pending:    []string{"DELETING", "RETRY"},
		Target:     []string{"IDLE"},
		Refresh:    resourceProjectDependentsDeletingRefreshFunc(ctx, projectID, conn),
		Timeout:    30 * time.Minute,
		MinTimeout: 30 * time.Second,
		Delay:      0,
	}

	_, err := stateConf.WaitForStateContext(ctx)

	if err != nil {
		tflog.Info(ctx, fmt.Sprintf("[ERROR] could not determine MongoDB project %s dependents status: %s", projectID, err.Error()))
	}

	_, err = conn.Projects.Delete(ctx, projectID)

	return err
}
