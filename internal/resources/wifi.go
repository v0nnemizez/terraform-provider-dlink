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

var _ resource.Resource = &WifiResource{}

type WifiResource struct {
	client *client.Client
}

func NewWifiResource() resource.Resource {
	return &WifiResource{}
}

type WifiResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Band         types.String `tfsdk:"band"`
	SSID         types.String `tfsdk:"ssid"`
	Password     types.String `tfsdk:"password"`
	Channel      types.Int64  `tfsdk:"channel"`
	Enabled      types.Bool   `tfsdk:"enabled"`
	SecurityMode types.String `tfsdk:"security_mode"`
}

func (r *WifiResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_wifi"
}

func (r *WifiResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages WiFi settings for a specific radio band on the D-Link router.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier equal to the band value.",
			},
			"band": schema.StringAttribute{
				Required:    true,
				Description: `Radio band to configure: "2.4GHz" or "5GHz".`,
			},
			"ssid": schema.StringAttribute{
				Required:    true,
				Description: "WiFi network name (SSID).",
			},
			"password": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "WiFi password (pre-shared key).",
			},
			"channel": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(0),
				Description: "WiFi channel. Use 0 for auto.",
			},
			"enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether the radio is enabled.",
			},
			"security_mode": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("WPA2-PSK"),
				Description: `Security mode, e.g. "WPA2-PSK" or "WPA3".`,
			},
		},
	}
}

func (r *WifiResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *WifiResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan WifiResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	settings := modelToWifiSettings(plan)
	if err := r.client.SetWifiSettings(settings); err != nil {
		resp.Diagnostics.AddError("Failed to set WiFi settings", err.Error())
		return
	}

	plan.ID = types.StringValue(plan.Band.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *WifiResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state WifiResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	settings, err := r.client.GetWifiSettings(state.Band.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read WiFi settings", err.Error())
		return
	}

	// Router does not support read-back — keep existing state as source of truth.
	if settings == nil {
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	state.SSID = types.StringValue(settings.SSID)
	state.Channel = types.Int64Value(int64(settings.Channel))
	state.Enabled = types.BoolValue(settings.Enabled)
	state.SecurityMode = types.StringValue(settings.SecurityMode)
	state.Password = types.StringValue(settings.Password)
	state.ID = types.StringValue(settings.Band)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *WifiResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan WifiResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	settings := modelToWifiSettings(plan)
	if err := r.client.SetWifiSettings(settings); err != nil {
		resp.Diagnostics.AddError("Failed to update WiFi settings", err.Error())
		return
	}

	plan.ID = types.StringValue(plan.Band.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *WifiResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// WiFi cannot be deleted, only disabled.
	var state WifiResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	settings := modelToWifiSettings(state)
	settings.Enabled = false
	if err := r.client.SetWifiSettings(settings); err != nil {
		resp.Diagnostics.AddError("Failed to disable WiFi (delete)", err.Error())
	}
}

func modelToWifiSettings(m WifiResourceModel) *client.WifiSettings {
	return &client.WifiSettings{
		Band:         m.Band.ValueString(),
		SSID:         m.SSID.ValueString(),
		Password:     m.Password.ValueString(),
		Channel:      int(m.Channel.ValueInt64()),
		Enabled:      m.Enabled.ValueBool(),
		SecurityMode: m.SecurityMode.ValueString(),
	}
}
