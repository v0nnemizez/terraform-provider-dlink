package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/local/terraform-provider-dlink/internal/client"
	"github.com/local/terraform-provider-dlink/internal/resources"
)

var _ provider.Provider = &DLinkProvider{}

type DLinkProvider struct{}

type DLinkProviderModel struct {
	Host     types.String `tfsdk:"host"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
	Endpoint types.String `tfsdk:"endpoint"`
}

func New() provider.Provider {
	return &DLinkProvider{}
}

func (p *DLinkProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "dlink"
}

func (p *DLinkProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Provider for managing D-Link DIR-X1530 routers via SOAP API.",
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				Required:    true,
				Description: "Router hostname or IP address (e.g. 192.168.0.1).",
			},
			"username": schema.StringAttribute{
				Required:    true,
				Description: "Router admin username.",
			},
			"password": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "Router admin password.",
			},
			"endpoint": schema.StringAttribute{
				Optional:    true,
				Description: `Full base URL to the SOAP API, e.g. "http://192.168.0.1/DHMAPI/". Defaults to http://<host>/DHMAPI/.`,
			},
		},
	}
}

func (p *DLinkProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config DLinkProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := config.Endpoint.ValueString()
	if endpoint == "" {
		endpoint = "http://" + config.Host.ValueString() + "/DHMAPI/"
	}

	c := client.NewClientWithEndpoint(
		endpoint,
		config.Username.ValueString(),
		config.Password.ValueString(),
	)

	if err := c.Login(); err != nil {
		resp.Diagnostics.AddError("Authentication failed", err.Error())
		return
	}

	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *DLinkProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		resources.NewWifiResource,
		resources.NewPortForwardResource,
		resources.NewParentalProfileResource,
		resources.NewFirewallRuleResource,
		resources.NewLanResource,
		resources.NewNetworkSettingsResource,
		resources.NewAdvNetworkSettingsResource,
	}
}

func (p *DLinkProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}
