package mongodbatlas

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	matlas "go.mongodb.org/atlas/mongodbatlas"
)

const (
	resourceName = "database_user"
	// TODO use from other resource
	errorConfigureSummary = "Unexpected Resource Configure Type"
	errorConfigure        = "Expected *MongoDBClient, got: %T. Please report this issue to the provider developers."
)

func NewDatabaseUserRS() resource.Resource {
	return &DatabaseUserRS{}
}

type DatabaseUserRS struct {
	client *MongoDBClient
}

type tfDatabaseUserRSModel struct {
	ProjectID        types.String `tfsdk:"project_id"`
	DatabaseName     types.String `tfsdk:"database_name"`
	AuthDatabaseName types.String `tfsdk:"auth_database_name"`
	Username         types.String `tfsdk:"username"`
	Password         types.String `tfsdk:"password"`
	X509Type         types.String `tfsdk:"x509_type"`
	LDAPAuthType     types.String `tfsdk:"ldap_auth_type"`
	AWSIAMType       types.String `tfsdk:"aws_iam_type"`
	Roles            types.Set    `tfsdk:"roles"`
	Labels           types.Set    `tfsdk:"labels"`
	Scopes           types.Set    `tfsdk:"scopes"`
}

type tfRoleModel struct {
	RoleName       types.String `tfsdk:"role_name"`
	CollectionName types.String `tfsdk:"collection_name"`
	DatabaseName   types.String `tfsdk:"database_name"`
}

type tfLabelModel struct {
	Key   types.String `tfsdk:"key"`
	Value types.String `tfsdk:"value"`
}

type tfScopeModel struct {
	Name types.String `tfsdk:"name"`
	Type types.String `tfsdk:"type"`
}

func (r *DatabaseUserRS) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = fmt.Sprintf("%s_%s", req.ProviderTypeName, resourceName)
}

func (r *DatabaseUserRS) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	client, err := configure(req.ProviderData)
	if err != nil {
		resp.Diagnostics.AddError(errorConfigureSummary, err.Error())
		return
	}
	r.client = client
}

// TODO: use from other resource, handle case were nil is provided?
func configure(providerData any) (*MongoDBClient, error) {
	if providerData == nil {
		return nil, nil
	}
	client, ok := providerData.(*MongoDBClient)
	if !ok {
		return nil, fmt.Errorf(errorConfigure, providerData)
	}
	return client, nil
}

func (r *DatabaseUserRS) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"database_name": schema.StringAttribute{
				Optional:           true,
				DeprecationMessage: "use auth_database_name instead",
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.Expressions{
						path.MatchRelative().AtParent().AtName("auth_database_name"),
					}...),
				},
			},
			"auth_database_name": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.Expressions{
						path.MatchRelative().AtParent().AtName("database_name"),
					}...),
				},
			},
			"username": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"password": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.Expressions{
						path.MatchRelative().AtParent().AtName("x509_type"),
						path.MatchRelative().AtParent().AtName("ldap_auth_type"),
						path.MatchRelative().AtParent().AtName("aws_iam_type"),
					}...),
				},
			},
			"x509_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("NONE"),
				Validators: []validator.String{
					stringvalidator.OneOf("NONE", "MANAGED", "CUSTOMER"),
				},
			},
			"ldap_auth_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("NONE"),
				Validators: []validator.String{
					stringvalidator.OneOf("NONE", "USER", "GROUP"),
				},
			},
			"aws_iam_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("NONE"),
				Validators: []validator.String{
					stringvalidator.OneOf("NONE", "USER", "ROLE"),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"roles": schema.SetNestedBlock{
				Validators: []validator.Set{setvalidator.IsRequired()},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"role_name": schema.StringAttribute{
							Optional: true,
							Computed: true,
						},
						"collection_name": schema.StringAttribute{
							Optional: true,
							Computed: true,
						},
						"database_name": schema.StringAttribute{
							Optional: true,
							Computed: true,
						},
					},
				},
			},
			"labels": schema.SetNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							Optional: true,
							Computed: true,
						},
						"value": schema.StringAttribute{
							Optional: true,
							Computed: true,
						},
					},
				},
			},
			"scopes": schema.SetNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Optional: true,
						},
						"type": schema.StringAttribute{
							Optional: true,
						},
					},
				},
			},
		},
	}
}

func (r *DatabaseUserRS) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var databaseUserPlan tfDatabaseUserRSModel

	conn := r.client.Atlas

	diags := req.Plan.Get(ctx, &databaseUserPlan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := databaseUserPlan.ProjectID.ValueString()

	if databaseUserPlan.DatabaseName.IsNull() && databaseUserPlan.AuthDatabaseName.IsNull() {
		resp.Diagnostics.AddError("one of database_name or auth_database_name must be configured", "")
		return
	}

	var authDatabaseName string
	if !databaseUserPlan.DatabaseName.IsNull() {
		authDatabaseName = databaseUserPlan.DatabaseName.ValueString()
	} else {
		authDatabaseName = databaseUserPlan.AuthDatabaseName.ValueString()
	}

	dbUserReq := &matlas.DatabaseUser{
		Roles:        convertRoles(ctx, &databaseUserPlan),
		GroupID:      projectID,
		Username:     databaseUserPlan.Username.ValueString(),
		Password:     databaseUserPlan.Password.ValueString(),
		X509Type:     databaseUserPlan.X509Type.ValueString(),
		AWSIAMType:   databaseUserPlan.AWSIAMType.ValueString(),
		LDAPAuthType: databaseUserPlan.LDAPAuthType.ValueString(),
		DatabaseName: authDatabaseName,
		Labels:       convertLabels(ctx, &databaseUserPlan),
		Scopes:       convertScopes(ctx, &databaseUserPlan),
	}

	dbUserRes, _, err := conn.DatabaseUsers.Create(ctx, projectID, dbUserReq)
	if err != nil {
		resp.Diagnostics.AddError("error creating database user", err.Error())
	}

	// TODO: update model with resp?

	diags = resp.State.Set(ctx, databaseUserPlan)

	resp.State.SetAttribute(ctx, path.Root("id"), encodeStateID(map[string]string{
		"project_id":         projectID,
		"username":           dbUserRes.Username,
		"auth_database_name": authDatabaseName,
	}))

	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

}

func (r *DatabaseUserRS) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var id string
	req.State.GetAttribute(ctx, path.Root("id"), &id)
	ids := decodeStateID(id)
	println(ids)
	// projectID := ids["project_id"]
	// username := ids["username"]
	// authDatabaseName := ids["auth_database_name"]
	// conn := r.client.Atlas
}

func (r *DatabaseUserRS) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
}

func (r *DatabaseUserRS) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {

}

func convertRoles(ctx context.Context, dbUser *tfDatabaseUserRSModel) []matlas.Role {
	var roles []tfRoleModel
	dbUser.Roles.ElementsAs(ctx, &roles, true)

	var result []matlas.Role
	if len(roles) > 0 {
		result = make([]matlas.Role, len(roles))
		for k, r := range roles {
			result[k] = matlas.Role{
				RoleName:       r.RoleName.ValueString(),
				DatabaseName:   r.DatabaseName.ValueString(),
				CollectionName: r.CollectionName.ValueString(),
			}
		}
	}
	return result
}

func convertLabels(ctx context.Context, dbUser *tfDatabaseUserRSModel) []matlas.Label {
	var labels []tfLabelModel
	dbUser.Labels.ElementsAs(ctx, &labels, true)

	var result []matlas.Label
	if len(labels) > 0 {
		result = make([]matlas.Label, len(labels))
		for k, r := range labels {
			result[k] = matlas.Label{
				Key:   r.Key.ValueString(),
				Value: r.Value.ValueString(),
			}
		}
	}
	return result
}

func convertScopes(ctx context.Context, dbUser *tfDatabaseUserRSModel) []matlas.Scope {
	var scope []tfScopeModel
	dbUser.Scopes.ElementsAs(ctx, &scope, true)

	var result []matlas.Scope
	if len(scope) > 0 {
		result = make([]matlas.Scope, len(scope))
		for k, r := range scope {
			result[k] = matlas.Scope{
				Type: r.Type.ValueString(),
				Name: r.Name.ValueString(),
			}
		}
	}
	return result
}
