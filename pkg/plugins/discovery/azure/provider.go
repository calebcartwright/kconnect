/*
Copyright 2020 The kconnect Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package azure

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/Azure/go-autorest/autorest"

	azid "github.com/fidelity/kconnect/pkg/azure/identity"
	"github.com/fidelity/kconnect/pkg/config"
	khttp "github.com/fidelity/kconnect/pkg/http"
	"github.com/fidelity/kconnect/pkg/oidc"
	"github.com/fidelity/kconnect/pkg/provider"
	"github.com/fidelity/kconnect/pkg/provider/common"
	"github.com/fidelity/kconnect/pkg/provider/discovery"
	"github.com/fidelity/kconnect/pkg/provider/identity"
	"github.com/fidelity/kconnect/pkg/provider/registry"
)

const (
	ProviderName = "aks"
	UsageExample = `  # Discover AKS clusters using Azure AD
	{{.CommandPath}} use aks --idp-protocol aad

	# Discover AKS clusters using file based credentials
	export AZURE_TENANT_ID="123455"
	export AZURE_CLIENT_ID="76849"
	export AZURE_CLIENT_SECRET="supersecret"
	{{.CommandPath}} use aks --idp-protocol az-env
  `
)

func init() {
	if err := registry.RegisterDiscoveryPlugin(&registry.DiscoveryPluginRegistration{
		PluginRegistration: registry.PluginRegistration{
			Name:                   ProviderName,
			UsageExample:           UsageExample,
			ConfigurationItemsFunc: ConfigurationItems,
		},
		CreateFunc:                 New,
		SupportedIdentityProviders: []string{"aad", "az-env"},
	}); err != nil {
		zap.S().Fatalw("Failed to register AKS discovery plugin", "error", err)
	}
}

// New will create a new Azure discovery plugin
func New(input *provider.PluginCreationInput) (discovery.Provider, error) {
	if input.HTTPClient == nil {
		return nil, provider.ErrHTTPClientRequired
	}

	return &aksClusterProvider{
		logger:      input.Logger,
		interactive: input.IsInteractice,
		httpClient:  input.HTTPClient,
	}, nil
}

type aksClusterProviderConfig struct {
	common.ClusterProviderConfig
	SubscriptionID   *string `json:"subscription-id"`
	SubscriptionName *string `json:"subscription-name"`
	ResourceGroup    *string `json:"resource-group"`
	Admin            bool    `json:"admin"`
	ClusterName      string  `json:"cluster-name"`
}

type aksClusterProvider struct {
	config     *aksClusterProviderConfig
	authorizer autorest.Authorizer

	httpClient  khttp.Client
	interactive bool
	logger      *zap.SugaredLogger
}

func (p *aksClusterProvider) Name() string {
	return ProviderName
}

func (p *aksClusterProvider) setup(cs config.ConfigurationSet, userID identity.Identity) error {
	cfg := &aksClusterProviderConfig{}
	if err := config.Unmarshall(cs, cfg); err != nil {
		return fmt.Errorf("unmarshalling config items into eksClusteProviderConfig: %w", err)
	}
	p.config = cfg

	// TODO: should we just return a AuthorizerIdentity from the aad provider?
	switch userID.(type) { //nolint:gocritic,gosimple
	case *oidc.Identity:
		id := userID.(*oidc.Identity)
		p.logger.Debugw("creating bearer authorizer")
		bearerAuth := autorest.NewBearerAuthorizer(id)
		p.authorizer = bearerAuth
	case *azid.AuthorizerIdentity:
		id := userID.(*azid.AuthorizerIdentity)
		p.authorizer = id.Authorizer()
	default:
		return ErrUnsupportedIdentity
	}

	return nil
}

func (p *aksClusterProvider) ListPreReqs() []*provider.PreReq {
	return []*provider.PreReq{}
}

func (p *aksClusterProvider) CheckPreReqs() error {
	return nil
}

// ConfigurationItems returns the configuration items for this provider
func ConfigurationItems(scopeTo string) (config.ConfigurationSet, error) {
	cs := config.NewConfigurationSet()

	cs.String(SubscriptionIDConfigItem, "", "The Azure subscription to use (specified by ID)")     //nolint: errcheck
	cs.String(SubscriptionNameConfigItem, "", "The Azure subscription to use (specified by name)") //nolint: errcheck
	cs.String(ResourceGroupConfigItem, "", "The Azure resource group to use")                      //nolint: errcheck
	cs.Bool(AdminConfigItem, false, "Generate admin user kubeconfig")                              //nolint: errcheck
	cs.String(ClusterNameConfigItem, "", "The name of the AKS cluster")                            //nolint: errcheck

	cs.SetShort(ResourceGroupConfigItem, "r") //nolint: errcheck

	return cs, nil
}
