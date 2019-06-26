package models

type LogInstance struct {
	Destination string
	URL         string
	Cookie      string
}

type SubsetStats struct {
	NumberOfElements  float64
	Average           float64
	StandardDeviation float64
}

type Experiment struct {
	Subsets map[string]SubsetStats
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
	LandingPath string
	TargetPath  string
	Subsets     map[string]bool
}

type ExperimentConfigurations struct {
	ExperimentConfigurationMap map[string]ExperimentConfiguration
}
