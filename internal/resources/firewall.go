package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/local/terraform-provider-dlink/internal/client"
)

var _ resource.Resource = &FirewallRuleResource{}

type FirewallRuleResource struct {
	client *client.Client
}

func NewFirewallRuleResource() resource.Resource {
	return &FirewallRuleResource{}
}

type FirewallRuleResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Enabled      types.Bool   `tfsdk:"enabled"`
	Schedule     types.String `tfsdk:"schedule"`
	SrcInterface types.String `tfsdk:"src_interface"`
	SrcIPStart   types.String `tfsdk:"src_ip_start"`
	SrcIPEnd     types.String `tfsdk:"src_ip_end"`
	DstInterface types.String `tfsdk:"dest_interface"`
	DstIPStart   types.String `tfsdk:"dest_ip_start"`
	DstIPEnd     types.String `tfsdk:"dest_ip_end"`
	Protocol     types.String `tfsdk:"protocol"`
	PortStart    types.Int64  `tfsdk:"port_start"`
	PortEnd      types.Int64  `tfsdk:"port_end"`
}

func (r *FirewallRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_firewall_rule"
}

func (r *FirewallRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a single IPv4 firewall rule on the D-Link router.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Rule ID (same as name).",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Rule name (must be unique on the router).",
			},
			"enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether the rule is active.",
			},
			"schedule": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("Always"),
				Description: "Schedule name. Defaults to \"Always\".",
			},
			"src_interface": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("WAN"),
				Description: `Source interface: "WAN" or "LAN".`,
			},
			"src_ip_start": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
				Description: "Start of the source IP address range.",
			},
			"src_ip_end": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
				Description: "End of the source IP address range.",
			},
			"dest_interface": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("LAN"),
				Description: `Destination interface: "LAN" or "WAN".`,
			},
			"dest_ip_start": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
				Description: "Start of the destination IP address range.",
			},
			"dest_ip_end": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(""),
				Description: "End of the destination IP address range.",
			},
			"protocol": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("TCP"),
				Description: `Protocol: "TCP", "UDP", "TCP/UDP", "ICMP", etc.`,
			},
			"port_start": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(0),
				Description: "Start of the port range.",
			},
			"port_end": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(0),
				Description: "End of the port range.",
			},
		},
	}
}

func (r *FirewallRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *FirewallRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan FirewallRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.ModifyFirewallRules(func(rules []client.FirewallRule) ([]client.FirewallRule, error) {
		for _, existing := range rules {
			if strings.EqualFold(existing.Name, plan.Name.ValueString()) {
				return nil, fmt.Errorf("a firewall rule named %q already exists", plan.Name.ValueString())
			}
		}
		return append(rules, firewallModelToRule(plan)), nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create firewall rule", err.Error())
		return
	}

	plan.ID = types.StringValue(plan.Name.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *FirewallRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state FirewallRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rules, _, err := r.client.GetFirewallRules()
	if err != nil {
		resp.Diagnostics.AddError("Failed to read firewall rules", err.Error())
		return
	}

	for _, rule := range rules {
		if strings.EqualFold(rule.Name, state.Name.ValueString()) {
			state.ID = types.StringValue(rule.Name)
			state.Name = types.StringValue(rule.Name)
			state.Enabled = types.BoolValue(rule.Enabled)
			state.Schedule = types.StringValue(rule.Schedule)
			state.SrcInterface = types.StringValue(rule.SrcInterface)
			state.SrcIPStart = types.StringValue(rule.SrcIPStart)
			state.SrcIPEnd = types.StringValue(rule.SrcIPEnd)
			state.DstInterface = types.StringValue(rule.DstInterface)
			state.DstIPStart = types.StringValue(rule.DstIPStart)
			state.DstIPEnd = types.StringValue(rule.DstIPEnd)
			state.Protocol = types.StringValue(rule.Protocol)
			state.PortStart = types.Int64Value(int64(rule.PortStart))
			state.PortEnd = types.Int64Value(int64(rule.PortEnd))
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}

	resp.State.RemoveResource(ctx)
}

func (r *FirewallRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan FirewallRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.ModifyFirewallRules(func(rules []client.FirewallRule) ([]client.FirewallRule, error) {
		for i, rule := range rules {
			if strings.EqualFold(rule.Name, plan.Name.ValueString()) {
				rules[i] = firewallModelToRule(plan)
				return rules, nil
			}
		}
		return append(rules, firewallModelToRule(plan)), nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update firewall rule", err.Error())
		return
	}

	plan.ID = types.StringValue(plan.Name.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *FirewallRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state FirewallRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.ModifyFirewallRules(func(rules []client.FirewallRule) ([]client.FirewallRule, error) {
		filtered := make([]client.FirewallRule, 0, len(rules))
		for _, rule := range rules {
			if !strings.EqualFold(rule.Name, state.Name.ValueString()) {
				filtered = append(filtered, rule)
			}
		}
		return filtered, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete firewall rule", err.Error())
	}
}

func firewallModelToRule(m FirewallRuleResourceModel) client.FirewallRule {
	return client.FirewallRule{
		Name:         m.Name.ValueString(),
		Enabled:      m.Enabled.ValueBool(),
		Schedule:     m.Schedule.ValueString(),
		SrcInterface: m.SrcInterface.ValueString(),
		SrcIPStart:   m.SrcIPStart.ValueString(),
		SrcIPEnd:     m.SrcIPEnd.ValueString(),
		DstInterface: m.DstInterface.ValueString(),
		DstIPStart:   m.DstIPStart.ValueString(),
		DstIPEnd:     m.DstIPEnd.ValueString(),
		Protocol:     m.Protocol.ValueString(),
		PortStart:    int(m.PortStart.ValueInt64()),
		PortEnd:      int(m.PortEnd.ValueInt64()),
	}
}
