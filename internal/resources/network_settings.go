package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/local/terraform-provider-dlink/internal/client"
)

var _ resource.Resource = &NetworkSettingsResource{}

type NetworkSettingsResource struct {
	client *client.Client
}

func NewNetworkSettingsResource() resource.Resource {
	return &NetworkSettingsResource{}
}

type NetworkSettingsResourceModel struct {
	ID              types.String `tfsdk:"id"`
	IPAddress       types.String `tfsdk:"ip_address"`
	SubnetMask      types.String `tfsdk:"subnet_mask"`
	DeviceName      types.String `tfsdk:"device_name"`
	LocalDomainName types.String `tfsdk:"local_domain_name"`
	IPRangeStart    types.Int64  `tfsdk:"ip_range_start"`
	IPRangeEnd      types.Int64  `tfsdk:"ip_range_end"`
	LeaseTime       types.Int64  `tfsdk:"lease_time"`
	Broadcast       types.Bool   `tfsdk:"broadcast"`
	DNSRelay        types.Bool   `tfsdk:"dns_relay"`
	MACAddress      types.String `tfsdk:"mac_address"`
}

func (r *NetworkSettingsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_network_settings"
}

func (r *NetworkSettingsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages DHCP and network settings on the D-Link router. " +
			"Note: ip_address and subnet_mask overlap with dlink_lan — keep them in sync.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Always \"network_settings\".",
			},
			"ip_address": schema.StringAttribute{
				Required:    true,
				Description: "Router LAN IP address. Should match dlink_lan.router_ip if both resources are used.",
			},
			"subnet_mask": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("255.255.255.0"),
				Description: "LAN subnet mask.",
			},
			"device_name": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
				Description: "Router hostname (e.g. \"R15-D105\").",
			},
			"local_domain_name": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
				Description: "Local domain name suffix for DHCP clients.",
			},
			"ip_range_start": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(1),
				Description: "Last octet of the DHCP range start address (e.g. 1 → 192.168.0.1).",
			},
			"ip_range_end": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(254),
				Description: "Last octet of the DHCP range end address (e.g. 254 → 192.168.0.254).",
			},
			"lease_time": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(10080),
				Description: "DHCP lease time in minutes. Defaults to 10080 (7 days).",
			},
			"broadcast": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether to enable broadcast. Defaults to false.",
			},
			"dns_relay": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether to enable DNS relay. Defaults to true.",
			},
			"mac_address": schema.StringAttribute{
				Computed:    true,
				Description: "Router LAN MAC address (read-only).",
			},
		},
	}
}

func (r *NetworkSettingsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *NetworkSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan NetworkSettingsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.SetNetworkSettings(modelToNetworkSettings(plan)); err != nil {
		resp.Diagnostics.AddError("Failed to set network settings", err.Error())
		return
	}

	settings, err := r.client.GetNetworkSettings()
	if err == nil && settings != nil {
		plan.MACAddress = types.StringValue(settings.MACAddress)
	}
	plan.ID = types.StringValue("network_settings")
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *NetworkSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state NetworkSettingsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	settings, err := r.client.GetNetworkSettings()
	if err != nil {
		resp.Diagnostics.AddError("Failed to read network settings", err.Error())
		return
	}
	if settings == nil {
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	state.IPAddress = types.StringValue(settings.IPAddress)
	state.SubnetMask = types.StringValue(settings.SubnetMask)
	state.DeviceName = types.StringValue(settings.DeviceName)
	state.LocalDomainName = types.StringValue(settings.LocalDomainName)
	state.IPRangeStart = types.Int64Value(int64(settings.IPRangeStart))
	state.IPRangeEnd = types.Int64Value(int64(settings.IPRangeEnd))
	state.LeaseTime = types.Int64Value(int64(settings.LeaseTime))
	state.Broadcast = types.BoolValue(settings.Broadcast)
	state.DNSRelay = types.BoolValue(settings.DNSRelay)
	state.MACAddress = types.StringValue(settings.MACAddress)
	state.ID = types.StringValue("network_settings")
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *NetworkSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan NetworkSettingsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.SetNetworkSettings(modelToNetworkSettings(plan)); err != nil {
		resp.Diagnostics.AddError("Failed to update network settings", err.Error())
		return
	}

	settings, err := r.client.GetNetworkSettings()
	if err == nil && settings != nil {
		plan.MACAddress = types.StringValue(settings.MACAddress)
	}
	plan.ID = types.StringValue("network_settings")
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *NetworkSettingsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Network settings cannot be deleted — removing from state only.
}

func modelToNetworkSettings(m NetworkSettingsResourceModel) client.NetworkSettings {
	return client.NetworkSettings{
		IPAddress:       m.IPAddress.ValueString(),
		SubnetMask:      m.SubnetMask.ValueString(),
		DeviceName:      m.DeviceName.ValueString(),
		LocalDomainName: m.LocalDomainName.ValueString(),
		IPRangeStart:    int(m.IPRangeStart.ValueInt64()),
		IPRangeEnd:      int(m.IPRangeEnd.ValueInt64()),
		LeaseTime:       int(m.LeaseTime.ValueInt64()),
		Broadcast:       m.Broadcast.ValueBool(),
		DNSRelay:        m.DNSRelay.ValueBool(),
	}
}
