package vampclientprovider

import (
	"errors"

	"github.com/magneticio/vampkubistcli/client"
	"github.com/magneticio/vampkubistcli/logging"

	"github.com/spf13/viper"
)

var URL string
var Token string
var APIVersion string
var Cert string

// TokenStore is initiliazed once and shared with all clients
var TokenStore client.TokenStore = &client.InMemoryTokenStore{}
var Project string
var Cluster string
var VirtualCluster string

// InitViperConfig used in tests so don't use this as a source of truth
func InitViperConfig(path string, configName string) {
	viper.SetConfigName(configName) // name of config file (without extension)
	viper.AddConfigPath(path)       // path to look for the config file in
	viper.AddConfigPath(".")        // optionally look for config in the working directory
	err := viper.ReadInConfig()     // Find and read the config file
	if err != nil {                 // Handle errors reading the config file
		logging.Error("Error config file: %s \n", err)
	}
}

// GetRestClient return current configured client
func GetRestClient() (*client.RestClient, error) {
	// TODO: Add client pooling
	URL = viper.GetString("url")
	Token = viper.GetString("token")
	APIVersion = viper.GetString("apiversion")
	Cert = viper.GetString("cert")
	Project = viper.GetString("project")
	Cluster = viper.GetString("cluster")
	VirtualCluster = viper.GetString("virtualcluster")

	restClient := client.NewRestClient(URL, Token, APIVersion, false, Cert, &TokenStore)
	if restClient == nil {
		return nil, errors.New("Rest Client can not be initiliazed")
	}
	return restClient, nil
}
