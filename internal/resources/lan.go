package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/local/terraform-provider-dlink/internal/client"
)

var _ resource.Resource = &LanResource{}

type LanResource struct {
	client *client.Client
}

func NewLanResource() resource.Resource {
	return &LanResource{}
}

type LanResourceModel struct {
	ID          types.String `tfsdk:"id"`
	RouterIP    types.String `tfsdk:"router_ip"`
	SubnetMask  types.String `tfsdk:"subnet_mask"`
	DHCPEnabled types.Bool   `tfsdk:"dhcp_enabled"`
	MACAddress  types.String `tfsdk:"mac_address"`
}

func (r *LanResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_lan"
}

func (r *LanResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages LAN settings on the D-Link router (IP address, subnet mask, DHCP). " +
			"WARNING: changing router_ip will disconnect the current session. Update the provider host accordingly.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Always \"lan\".",
			},
			"router_ip": schema.StringAttribute{
				Required:    true,
				Description: "Router LAN IP address (e.g. \"192.168.0.1\").",
			},
			"subnet_mask": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("255.255.255.0"),
				Description: "LAN subnet mask. Defaults to \"255.255.255.0\".",
			},
			"dhcp_enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether the DHCP server is enabled. Defaults to true.",
			},
			"mac_address": schema.StringAttribute{
				Computed:    true,
				Description: "Router LAN MAC address (read-only, assigned by hardware).",
			},
		},
	}
}

func (r *LanResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *LanResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan LanResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.SetLanSettings(client.LanSettings{
		RouterIP:    plan.RouterIP.ValueString(),
		SubnetMask:  plan.SubnetMask.ValueString(),
		DHCPEnabled: plan.DHCPEnabled.ValueBool(),
	}); err != nil {
		resp.Diagnostics.AddError("Failed to set LAN settings", err.Error())
		return
	}

	// Read back to get computed MAC address.
	settings, err := r.client.GetLanSettings()
	if err == nil && settings != nil {
		plan.MACAddress = types.StringValue(settings.MACAddress)
	}

	plan.ID = types.StringValue("lan")
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *LanResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state LanResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	settings, err := r.client.GetLanSettings()
	if err != nil {
		resp.Diagnostics.AddError("Failed to read LAN settings", err.Error())
		return
	}
	if settings == nil {
		// Router returned empty body — keep existing state.
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	state.RouterIP = types.StringValue(settings.RouterIP)
	state.SubnetMask = types.StringValue(settings.SubnetMask)
	state.DHCPEnabled = types.BoolValue(settings.DHCPEnabled)
	state.MACAddress = types.StringValue(settings.MACAddress)
	state.ID = types.StringValue("lan")
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *LanResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan LanResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.SetLanSettings(client.LanSettings{
		RouterIP:    plan.RouterIP.ValueString(),
		SubnetMask:  plan.SubnetMask.ValueString(),
		DHCPEnabled: plan.DHCPEnabled.ValueBool(),
	}); err != nil {
		resp.Diagnostics.AddError("Failed to update LAN settings", err.Error())
		return
	}

	// Read back to get computed MAC address.
	settings, err := r.client.GetLanSettings()
	if err == nil && settings != nil {
		plan.MACAddress = types.StringValue(settings.MACAddress)
	}

	plan.ID = types.StringValue("lan")
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *LanResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// LAN settings cannot be deleted — removing from state only.
}
