package configurator

import (
	"sync"

	"github.com/magneticio/vampkubistcli/client"
	"github.com/magneticio/vampkubistcli/logging"
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

type stats struct {
	NumberOfElements  float64
	Average           float64
	StandardDeviation float64
}

type Experiment struct {
	SubsetStats map[string]stats
}

var Config config

var experiments sync.Map

func query() error {
	restClient := client.NewRestClient(Config.Url, Config.Token, Config.APIVersion, logging.Verbose, Config.Cert)
	return nil
}
