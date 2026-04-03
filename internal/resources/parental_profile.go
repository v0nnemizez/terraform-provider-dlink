package resources

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/local/terraform-provider-dlink/internal/client"
)

var _ resource.Resource = &ParentalProfileResource{}

// ParentalProfileResource manages a single D-Link parental control profile.
type ParentalProfileResource struct {
	client *client.Client
}

// NewParentalProfileResource returns a new instance of the resource.
func NewParentalProfileResource() resource.Resource {
	return &ParentalProfileResource{}
}

// ParentalProfileResourceModel is the Terraform state model for dlink_parental_profile.
type ParentalProfileResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	FilterEnabled   types.Bool   `tfsdk:"filter_enabled"`
	BlockedDomains  types.List   `tfsdk:"blocked_domains"`
	AllowSlowAccess types.Bool   `tfsdk:"allow_slow_access"`
	Devices         types.List   `tfsdk:"devices"`
}

// BlockedDomainModel is the nested object model for a single blocked domain entry.
type BlockedDomainModel struct {
	Title  types.String `tfsdk:"title"`
	Domain types.String `tfsdk:"domain"`
}

// blockedDomainAttrTypes defines the attribute types for the blocked_domains list element.
var blockedDomainAttrTypes = map[string]attr.Type{
	"title":  types.StringType,
	"domain": types.StringType,
}

func (r *ParentalProfileResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_parental_profile"
}

func (r *ParentalProfileResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	emptyList, _ := types.ListValue(types.ObjectType{AttrTypes: blockedDomainAttrTypes}, []attr.Value{})
	emptyStringList, _ := types.ListValue(types.StringType, []attr.Value{})

	resp.Schema = schema.Schema{
		Description: "Manages a parental control profile on the D-Link router.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The UUID assigned to the profile by this provider.",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Display name for the parental profile.",
			},
			"filter_enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether the domain filter is active for this profile.",
			},
			"blocked_domains": schema.ListNestedAttribute{
				Optional:    true,
				Computed:    true,
				Default:     listdefault.StaticValue(emptyList),
				Description: "List of domains blocked by this profile.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"title": schema.StringAttribute{
							Required:    true,
							Description: "Human-readable label for the blocked domain entry.",
						},
						"domain": schema.StringAttribute{
							Required:    true,
							Description: "Domain name to block (e.g. youtube.com).",
						},
					},
				},
			},
			"allow_slow_access": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether to allow slow (throttled) access for this profile.",
			},
			"devices": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				Default:     listdefault.StaticValue(emptyStringList),
				ElementType: types.StringType,
				Description: "MAC addresses of devices assigned to this profile (e.g. \"8a:5b:39:45:2c:19\").",
			},
		},
	}
}

func (r *ParentalProfileResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type",
			fmt.Sprintf("Expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *ParentalProfileResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ParentalProfileResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	uuid := generateUUID()
	profile := modelToProfile(ctx, plan, uuid)

	if err := r.client.SetParentalProfile(profile, false); err != nil {
		resp.Diagnostics.AddError("Failed to create parental profile", err.Error())
		return
	}

	// The router may assign its own UUID instead of the one we sent.
	// Fetch all profiles and find ours by name to get the actual UUID.
	actualUUID, err := r.client.FindParentalProfileUUIDByName(plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to confirm created parental profile", err.Error())
		return
	}
	if actualUUID != "" {
		uuid = actualUUID
	}

	plan.ID = types.StringValue(uuid)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ParentalProfileResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ParentalProfileResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	profiles, err := r.client.GetParentalProfiles()
	if err != nil {
		resp.Diagnostics.AddError("Failed to read parental profiles", err.Error())
		return
	}

	for _, p := range profiles {
		if strings.EqualFold(p.UUID, state.ID.ValueString()) {
			state.Name = types.StringValue(p.Name)
			state.FilterEnabled = types.BoolValue(p.FilterEnabled)
			state.AllowSlowAccess = types.BoolValue(p.AllowSlowAccess)

			domainObjects := make([]attr.Value, 0, len(p.BlockedDomains))
			for _, d := range p.BlockedDomains {
				obj, diags := types.ObjectValue(blockedDomainAttrTypes, map[string]attr.Value{
					"title":  types.StringValue(d.Title),
					"domain": types.StringValue(d.Domain),
				})
				resp.Diagnostics.Append(diags...)
				if resp.Diagnostics.HasError() {
					return
				}
				domainObjects = append(domainObjects, obj)
			}
			list, diags := types.ListValue(types.ObjectType{AttrTypes: blockedDomainAttrTypes}, domainObjects)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			state.BlockedDomains = list

			deviceValues := make([]attr.Value, 0, len(p.Devices))
			for _, mac := range p.Devices {
				deviceValues = append(deviceValues, types.StringValue(mac))
			}
			deviceList, diags := types.ListValue(types.StringType, deviceValues)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			state.Devices = deviceList

			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}

	// Profile no longer exists on the router.
	resp.State.RemoveResource(ctx)
}

func (r *ParentalProfileResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ParentalProfileResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state ParentalProfileResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Preserve the existing UUID on update.
	profile := modelToProfile(ctx, plan, state.ID.ValueString())

	if err := r.client.SetParentalProfile(profile, false); err != nil {
		resp.Diagnostics.AddError("Failed to update parental profile", err.Error())
		return
	}

	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ParentalProfileResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ParentalProfileResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	profile := modelToProfile(ctx, state, state.ID.ValueString())

	if err := r.client.SetParentalProfile(profile, true); err != nil {
		resp.Diagnostics.AddError("Failed to delete parental profile", err.Error())
	}
}

// modelToProfile converts the Terraform state model to the client profile type.
func modelToProfile(ctx context.Context, m ParentalProfileResourceModel, uuid string) client.ParentalProfile {
	var domainModels []BlockedDomainModel
	m.BlockedDomains.ElementsAs(ctx, &domainModels, false)

	domains := make([]client.BlockedDomain, 0, len(domainModels))
	for _, d := range domainModels {
		domains = append(domains, client.BlockedDomain{
			Title:  d.Title.ValueString(),
			Domain: d.Domain.ValueString(),
		})
	}

	var deviceStrings []basetypes.StringValue
	m.Devices.ElementsAs(ctx, &deviceStrings, false)
	devices := make([]string, 0, len(deviceStrings))
	for _, d := range deviceStrings {
		devices = append(devices, d.ValueString())
	}

	return client.ParentalProfile{
		UUID:            uuid,
		Name:            m.Name.ValueString(),
		FilterEnabled:   m.FilterEnabled.ValueBool(),
		BlockedDomains:  domains,
		AllowSlowAccess: m.AllowSlowAccess.ValueBool(),
		Devices:         devices,
	}
}

// generateUUID produces a random UUID v4 string in the format used by the D-Link router.
func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08X-%04X-%04X-%04X-%012X",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
