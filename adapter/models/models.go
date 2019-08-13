package models

type LogInstance struct {
	Timestamp         int64
	Destination       string
	DestinationPort   string
	DestinationLabels map[string]string
	Values            map[string]interface{}
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

type Instance struct {
	APIVersion string            `yaml:"apiVersion,omitempty" json:"apiVersion,omitempty"`
	Kind       string            `yaml:"kind,omitempty" json:"kind,omitempty"`
	Metadata   map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	Spec       InstanceSpec      `yaml:"spec,omitempty" json:"spec,omitempty"`
}

type InstanceSpec struct {
	Template         string         `yaml:"template,omitempty" json:"template,omitempty"`
	CompiledTemplate string         `yaml:"compiledTemplate,omitempty" json:"compiledTemplate,omitempty"`
	Params           InstanceParams `yaml:"params,omitempty" json:"params,omitempty"`
}

type InstanceParams struct {
	Severity  string            `yaml:"severity,omitempty" json:"severity,omitempty"`
	Timestamp string            `yaml:"timestamp,omitempty" json:"timestamp,omitempty"`
	Variables map[string]string `yaml:"variables,omitempty" json:"variables,omitempty"`
}
