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

var _ resource.Resource = &QuickVPNResource{}

type QuickVPNResource struct {
	client *client.Client
}

func NewQuickVPNResource() resource.Resource {
	return &QuickVPNResource{}
}

type QuickVPNResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Enabled      types.Bool   `tfsdk:"enabled"`
	PSK          types.String `tfsdk:"psk"`
	Username     types.String `tfsdk:"username"`
	Password     types.String `tfsdk:"password"`
	AuthProtocol types.String `tfsdk:"auth_protocol"`
	MPPE         types.String `tfsdk:"mppe"`
}

func (r *QuickVPNResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_quick_vpn"
}

func (r *QuickVPNResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages the D-Link QuickVPN (L2TP/IPsec) configuration.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Always \"quick_vpn\".",
			},
			"enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether QuickVPN is enabled. Defaults to false.",
			},
			"psk": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "Pre-Shared Key (PSK) for L2TP/IPsec authentication.",
			},
			"username": schema.StringAttribute{
				Required:    true,
				Description: "VPN username.",
			},
			"password": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "VPN password.",
			},
			"auth_protocol": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("MSCHAPv2"),
				Description: "Authentication protocol. Defaults to \"MSCHAPv2\".",
			},
			"mppe": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("None"),
				Description: "MPPE encryption setting. Defaults to \"None\".",
			},
		},
	}
}

func (r *QuickVPNResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *QuickVPNResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan QuickVPNResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.SetQuickVPNSettings(modelToQuickVPN(plan)); err != nil {
		resp.Diagnostics.AddError("Failed to set QuickVPN settings", err.Error())
		return
	}

	plan.ID = types.StringValue("quick_vpn")
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *QuickVPNResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state QuickVPNResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	settings, err := r.client.GetQuickVPNSettings()
	if err != nil {
		resp.Diagnostics.AddError("Failed to read QuickVPN settings", err.Error())
		return
	}
	if settings == nil {
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	state.Enabled = types.BoolValue(settings.Enabled)
	state.Username = types.StringValue(settings.Username)
	state.AuthProtocol = types.StringValue(settings.AuthProtocol)
	state.MPPE = types.StringValue(settings.MPPE)
	// Password and PSK are returned encrypted by the router — keep the
	// plaintext values already in state to avoid permanent drift.
	state.ID = types.StringValue("quick_vpn")
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *QuickVPNResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan QuickVPNResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.SetQuickVPNSettings(modelToQuickVPN(plan)); err != nil {
		resp.Diagnostics.AddError("Failed to update QuickVPN settings", err.Error())
		return
	}

	plan.ID = types.StringValue("quick_vpn")
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *QuickVPNResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state QuickVPNResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Disable VPN on the router rather than leaving it enabled without Terraform management.
	if err := r.client.SetQuickVPNSettings(client.QuickVPNSettings{
		Enabled:      false,
		Username:     state.Username.ValueString(),
		Password:     state.Password.ValueString(),
		PSK:          state.PSK.ValueString(),
		AuthProtocol: state.AuthProtocol.ValueString(),
		MPPE:         state.MPPE.ValueString(),
	}); err != nil {
		resp.Diagnostics.AddError("Failed to disable QuickVPN", err.Error())
	}
}

func modelToQuickVPN(m QuickVPNResourceModel) client.QuickVPNSettings {
	return client.QuickVPNSettings{
		Enabled:      m.Enabled.ValueBool(),
		PSK:          m.PSK.ValueString(),
		Username:     m.Username.ValueString(),
		Password:     m.Password.ValueString(),
		AuthProtocol: m.AuthProtocol.ValueString(),
		MPPE:         m.MPPE.ValueString(),
	}
}
