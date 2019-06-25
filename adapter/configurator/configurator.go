package configurator

import (
	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/models"
)

type config struct {
	Url            string `yaml:"url,omitempty" json:"url,omitempty"`
	Cert           string `yaml:"cert,omitempty" json:"cert,omitempty"`
	Username       string `yaml:"username,omitempty" json:"username,omitempty"`
	Token          string `yaml:"token,omitempty" json:"token,omitempty"`
	Project        string `yaml:"project,omitempty" json:"project,omitempty"`
	Cluster        string `yaml:"cluster,omitempty" json:"cluster,omitempty"`
	VirtualCluster string `yaml:"virtualcluster,omitempty" json:"virtualcluster,omitempty"`
	APIVersion     string `yaml:"apiversion,omitempty" json:"apiversion,omitempty"`
}

var Config config

// var Experiments map[string]Experiment

var ExperimentConfigurations map[string]*models.ExperimentConfiguration

func query() error {
	// restClient := client.NewRestClient(Config.Url, Config.Token, Config.APIVersion, logging.Verbose, Config.Cert)
	return nil
}
