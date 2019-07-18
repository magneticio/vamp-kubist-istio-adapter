package models

type LogInstance struct {
	Destination        string
	URL                string
	Cookie             string
	ResponseCode       string
	Latency            string
	DestinationPort    string
	DestinationVersion string
	DestinationLabels  map[string]string
}

type SubsetStats struct {
	NumberOfElements  float64
	Average           float64
	StandardDeviation float64
}

type ExperimentStats struct {
	Subsets map[string]SubsetStats
}

type ExperimentStatsGroup struct {
	ExperimentStatsMap map[string]ExperimentStats
}

type SubsetLogs struct {
	UserLogs map[string]int
}

type ExperimentLogs struct {
	SubsetLogs map[string]SubsetLogs
}

type ExperimentLoggers struct {
	ExperimentLogs map[string]ExperimentLogs
}

type ExperimentConfiguration struct {
	Subsets map[string]string
}

type ExperimentConfigurations struct {
	ExperimentConfigurationMap map[string]ExperimentConfiguration
}

//-------------------------//

type ExperimentDestination struct {
	Port        int64  `yaml:"port,omitempty" json:"port,omitempty"`
	Subset      string `yaml:"subset,omitempty" json:"subset,omitempty"`
	Target      string `yaml:"target,omitempty" json:"target,omitempty"`
	Destination string `yaml:"destination,omitempty" json:"destination,omitempty"`
}
type ExperimentSpecification struct {
	Destinations []ExperimentDestination `yaml:"destinations,omitempty" json:"destinations,omitempty"`
}

type Experiment struct {
	Name          string                  `yaml:"name,omitempty" json:"name,omitempty"`
	Specification ExperimentSpecification `yaml:"specification,omitempty" json:"specification,omitempty"`
}
