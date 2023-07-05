package provider

// Some issues when migrating provider:
// DefaultFunc: MultiEnvDefaultFunc not supported anymore in Framework [https://discuss.hashicorp.com/t/muxing-upgraded-tfsdk-and-framework-provider-with-default-provider-configuration/43945]
// terraform-plugin-sdk/v2/helper/logging - logging.NewTransport no longer supported [https://discuss.hashicorp.com/t/frameworks-alternative-to-terraform-plugin-sdk-v2-helper-logging/52371/2]
import (
	"context"
	"fmt"
	"log"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-mux/tf5to6server"
	"github.com/hashicorp/terraform-plugin-mux/tf6muxserver"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"

	mongodbatlasSDKv2 "github.com/mongodb/terraform-provider-mongodbatlas/mongodbatlas"
)

var TestAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"mongodbatlas": func() (tfprotov6.ProviderServer, error) {

		upgradedSdkProvider, err := tf5to6server.UpgradeServer(context.Background(), mongodbatlasSDKv2.Provider().GRPCProvider)
		if err != nil {
			log.Fatal(err)
		}
		providers := []func() tfprotov6.ProviderServer{
			func() tfprotov6.ProviderServer {
				return upgradedSdkProvider
			},
			providerserver.NewProtocol6(New()()),
		}

		muxServer, err := tf6muxserver.NewMuxServer(context.Background(), providers...)

		if err != nil {
			return nil, err
		}

		return muxServer.ProviderServer(), nil
	},
}

func TestMuxServer(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"mongodbatlas": func() (tfprotov6.ProviderServer, error) {
				ctx := context.Background()

				upgradedSdkServer, err := tf5to6server.UpgradeServer(
					ctx,
					mongodbatlasSDKv2.Provider().GRPCProvider,
				)

				if err != nil {
					return nil, err
				}

				providers := []func() tfprotov6.ProviderServer{
					func() tfprotov6.ProviderServer {
						return upgradedSdkServer
					},
					providerserver.NewProtocol6(New()()),
				}

				muxServer, err := tf6muxserver.NewMuxServer(ctx, providers...)

				if err != nil {
					return nil, err
				}

				return muxServer.ProviderServer(), nil
			},
		},
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprint(`resource mongodbatlas_example "test" {
					configurable_attribute = "config_attr_val"
					
				}`),
			},
		},
	})
}
