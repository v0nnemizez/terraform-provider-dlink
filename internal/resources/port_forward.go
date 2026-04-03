package resources

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/local/terraform-provider-dlink/internal/client"
)

var _ resource.Resource = &PortForwardResource{}

type PortForwardResource struct {
	client *client.Client
}

func NewPortForwardResource() resource.Resource {
	return &PortForwardResource{}
}

type PortForwardResourceModel struct {
	ID       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	Protocol types.String `tfsdk:"protocol"`
	Port     types.Int64  `tfsdk:"port"`
	LocalIP  types.String `tfsdk:"local_ip"`
	Schedule types.String `tfsdk:"schedule"`
	Enabled  types.Bool   `tfsdk:"enabled"`
}

func (r *PortForwardResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_port_forward"
}

func (r *PortForwardResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a single port forwarding rule on the D-Link router. " +
			"Note: this router maps the same port on both WAN and LAN sides.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Composite ID: <name>/<port>.",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Rule description (must be unique on the router).",
			},
			"protocol": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("TCP"),
				Description: `Protocol: "TCP", "UDP", or "TCP/UDP".`,
			},
			"port": schema.Int64Attribute{
				Required:    true,
				Description: "Port number (same on WAN and LAN sides).",
			},
			"local_ip": schema.StringAttribute{
				Required:    true,
				Description: "LAN IP address of the destination host.",
			},
			"schedule": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("Always"),
				Description: "Schedule name. Defaults to \"Always\".",
			},
			"enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether the rule is active.",
			},
		},
	}
}

func (r *PortForwardResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *PortForwardResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan PortForwardResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.ModifyPortForwardingRules(func(rules []client.PortForwardRule) ([]client.PortForwardRule, error) {
		for _, existing := range rules {
			if strings.EqualFold(existing.Name, plan.Name.ValueString()) {
				return nil, fmt.Errorf("a rule named %q already exists", plan.Name.ValueString())
			}
		}
		return append(rules, modelToRule(plan)), nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create port forward rule", err.Error())
		return
	}

	plan.ID = types.StringValue(ruleID(plan.Name.ValueString(), int(plan.Port.ValueInt64())))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *PortForwardResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state PortForwardResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rules, err := r.client.GetPortForwardingRules()
	if err != nil {
		resp.Diagnostics.AddError("Failed to read port forward rules", err.Error())
		return
	}

	for _, rule := range rules {
		if strings.EqualFold(rule.Name, state.Name.ValueString()) {
			state.Name = types.StringValue(rule.Name)
			state.Protocol = types.StringValue(rule.Protocol)
			state.Port = types.Int64Value(int64(rule.Port))
			state.LocalIP = types.StringValue(rule.LocalIP)
			state.Schedule = types.StringValue(rule.Schedule)
			state.Enabled = types.BoolValue(rule.Enabled)
			state.ID = types.StringValue(ruleID(rule.Name, rule.Port))
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}

	resp.State.RemoveResource(ctx)
}

func (r *PortForwardResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan PortForwardResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.ModifyPortForwardingRules(func(rules []client.PortForwardRule) ([]client.PortForwardRule, error) {
		for i, rule := range rules {
			if strings.EqualFold(rule.Name, plan.Name.ValueString()) {
				rules[i] = modelToRule(plan)
				return rules, nil
			}
		}
		return append(rules, modelToRule(plan)), nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update port forward rule", err.Error())
		return
	}

	plan.ID = types.StringValue(ruleID(plan.Name.ValueString(), int(plan.Port.ValueInt64())))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *PortForwardResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state PortForwardResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.ModifyPortForwardingRules(func(rules []client.PortForwardRule) ([]client.PortForwardRule, error) {
		filtered := make([]client.PortForwardRule, 0, len(rules))
		for _, rule := range rules {
			if !strings.EqualFold(rule.Name, state.Name.ValueString()) {
				filtered = append(filtered, rule)
			}
		}
		return filtered, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete port forward rule", err.Error())
	}
}

func modelToRule(m PortForwardResourceModel) client.PortForwardRule {
	return client.PortForwardRule{
		Name:     m.Name.ValueString(),
		Protocol: m.Protocol.ValueString(),
		Port:     int(m.Port.ValueInt64()),
		LocalIP:  m.LocalIP.ValueString(),
		Schedule: m.Schedule.ValueString(),
		Enabled:  m.Enabled.ValueBool(),
	}
}

func ruleID(name string, port int) string {
	return name + "/" + strconv.Itoa(port)
}
