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

var _ resource.Resource = &AdvNetworkSettingsResource{}

type AdvNetworkSettingsResource struct {
	client *client.Client
}

func NewAdvNetworkSettingsResource() resource.Resource {
	return &AdvNetworkSettingsResource{}
}

type AdvNetworkSettingsResourceModel struct {
	ID            types.String `tfsdk:"id"`
	UPNP          types.Bool   `tfsdk:"upnp"`
	MulticastIPv4 types.Bool   `tfsdk:"multicast_ipv4"`
	MulticastIPv6 types.Bool   `tfsdk:"multicast_ipv6"`
	WANPortSpeed  types.String `tfsdk:"wan_port_speed"`
}

func (r *AdvNetworkSettingsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_adv_network_settings"
}

func (r *AdvNetworkSettingsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages advanced network settings on the D-Link router (UPnP, multicast, WAN port speed).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Always \"adv_network_settings\".",
			},
			"upnp": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether UPnP is enabled. Defaults to true.",
			},
			"multicast_ipv4": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether IPv4 multicast is enabled. Defaults to true.",
			},
			"multicast_ipv6": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether IPv6 multicast is enabled. Defaults to true.",
			},
			"wan_port_speed": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("Auto"),
				Description: "WAN port speed. Defaults to \"Auto\".",
			},
		},
	}
}

func (r *AdvNetworkSettingsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *AdvNetworkSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AdvNetworkSettingsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.SetAdvNetworkSettings(modelToAdvNetworkSettings(plan)); err != nil {
		resp.Diagnostics.AddError("Failed to set advanced network settings", err.Error())
		return
	}

	plan.ID = types.StringValue("adv_network_settings")
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AdvNetworkSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state AdvNetworkSettingsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	settings, err := r.client.GetAdvNetworkSettings()
	if err != nil {
		resp.Diagnostics.AddError("Failed to read advanced network settings", err.Error())
		return
	}
	if settings == nil {
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	state.UPNP = types.BoolValue(settings.UPNP)
	state.MulticastIPv4 = types.BoolValue(settings.MulticastIPv4)
	state.MulticastIPv6 = types.BoolValue(settings.MulticastIPv6)
	state.WANPortSpeed = types.StringValue(settings.WANPortSpeed)
	state.ID = types.StringValue("adv_network_settings")
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *AdvNetworkSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan AdvNetworkSettingsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.SetAdvNetworkSettings(modelToAdvNetworkSettings(plan)); err != nil {
		resp.Diagnostics.AddError("Failed to update advanced network settings", err.Error())
		return
	}

	plan.ID = types.StringValue("adv_network_settings")
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AdvNetworkSettingsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Advanced network settings cannot be deleted — removing from state only.
}

func modelToAdvNetworkSettings(m AdvNetworkSettingsResourceModel) client.AdvNetworkSettings {
	return client.AdvNetworkSettings{
		UPNP:          m.UPNP.ValueBool(),
		MulticastIPv4: m.MulticastIPv4.ValueBool(),
		MulticastIPv6: m.MulticastIPv6.ValueBool(),
		WANPortSpeed:  m.WANPortSpeed.ValueString(),
	}
}
