package vampclientprovider

import (
	"errors"

	"github.com/magneticio/vampkubistcli/client"
	"github.com/magneticio/vampkubistcli/logging"

	"github.com/spf13/viper"
)

type IVampClientProvider interface {
	GetVirtualCluster() string
	SetVirtualCluster(virtualcluster string)
	GetConfigValues() map[string]string
	GetRestClient() (client.IRestClient, error)
}

type VampClientProvider struct {
	URL            string
	Token          string
	APIVersion     string
	Cert           string
	TokenStore     client.TokenStore
	Project        string
	Cluster        string
	VirtualCluster string
}

// InitViperConfig used in tests so don't use this as a source of truth
func InitViperConfig(path string, configName string) error {
	viper.SetConfigName(configName) // name of config file (without extension)
	viper.AddConfigPath(path)       // path to look for the config file in
	viper.AddConfigPath(".")        // optionally look for config in the working directory
	err := viper.ReadInConfig()     // Find and read the config file
	if err != nil {                 // Handle errors reading the config file
		logging.Error("Error config file: %s \n", err)
		return err
	}
	return nil
}

func New() IVampClientProvider {
	return &VampClientProvider{
		URL:            viper.GetString("url"),
		Token:          viper.GetString("token"),
		APIVersion:     viper.GetString("apiversion"),
		Cert:           viper.GetString("cert"),
		TokenStore:     &client.InMemoryTokenStore{},
		Project:        viper.GetString("project"),
		Cluster:        viper.GetString("cluster"),
		VirtualCluster: viper.GetString("virtualcluster"),
	}
}

func (vpc *VampClientProvider) GetVirtualCluster() string {
	if vpc.VirtualCluster == "" {
		vpc.VirtualCluster = viper.GetString("virtualcluster")
	}
	return vpc.VirtualCluster
}

func (vpc *VampClientProvider) SetVirtualCluster(virtualcluster string) {
	vpc.VirtualCluster = virtualcluster
}

// GetConfigValues return configuration values for rest client
func (vpc *VampClientProvider) GetConfigValues() map[string]string {
	values := make(map[string]string)
	values["project"] = vpc.Project
	values["cluster"] = vpc.Cluster
	values["virtual_cluster"] = vpc.VirtualCluster
	return values
}

// checkAndSetParameters is a temporary solution
// to early initiliazed client provider problem
func (vpc *VampClientProvider) checkAndSetParameters() {
	if vpc.URL == "" {
		vpc.URL = viper.GetString("url")
		vpc.Token = viper.GetString("token")
		vpc.APIVersion = viper.GetString("apiversion")
		vpc.Cert = viper.GetString("cert")
		vpc.Project = viper.GetString("project")
		vpc.Cluster = viper.GetString("cluster")
		vpc.VirtualCluster = viper.GetString("virtualcluster")
	}
}

// GetRestClient return current configured client
func (vpc *VampClientProvider) GetRestClient() (client.IRestClient, error) {
	vpc.checkAndSetParameters()
	restClient := client.NewRestClient(vpc.URL, vpc.Token, vpc.APIVersion, false, vpc.Cert, &vpc.TokenStore)
	if restClient == nil {
		return nil, errors.New("Rest Client can not be initiliazed")
	}
	return restClient, nil
}
