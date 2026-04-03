package main

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/local/terraform-provider-dlink/internal/provider"
)

func main() {
	err := providerserver.Serve(context.Background(), provider.New, providerserver.ServeOpts{
		Address: "registry.opentofu.org/v0nNemizez/dlink",
	})
	if err != nil {
		log.Fatal(err)
	}
}
