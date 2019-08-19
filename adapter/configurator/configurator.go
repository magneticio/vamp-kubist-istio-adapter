package configurator

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/models"
	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/vampclientprovider"
	"github.com/magneticio/vampkubistcli/logging"
	clientmodels "github.com/magneticio/vampkubistcli/models"
)

var ExperimentConfigurations0 models.ExperimentConfigurations
var ExperimentConfigurations1 models.ExperimentConfigurations

var activeConfigurationID int32

const RefreshPeriod = 30 * time.Second

func GetExperimentConfigurations() *models.ExperimentConfigurations {
	if atomic.LoadInt32(&activeConfigurationID) == 0 {
		return &ExperimentConfigurations0
	}
	return &ExperimentConfigurations1
}

func ParseExperimentConfiguration(sourceAsJson string) (*models.ExperimentConfigurations, error) {
	var experiments []models.Experiment
	err := json.Unmarshal([]byte(sourceAsJson), &experiments)
	if err != nil {
		return nil, err
	}
	experimentConfigurationMap := make(map[string]models.ExperimentConfiguration)
	for _, experiment := range experiments {
		subsets := make(map[string]string)
		for _, destination := range experiment.Specification.Destinations {
			portString := strconv.FormatInt(destination.Port, 10)
			key := destination.Destination + "-" + portString + "-" + destination.Subset
			subsets[key] = destination.Target
		}
		experimentConfigurationMap[experiment.Name] = models.ExperimentConfiguration{
			Subsets: subsets,
		}
	}
	return &models.ExperimentConfigurations{
		ExperimentConfigurationMap: experimentConfigurationMap,
	}, nil
}

// GenerateNewExperimentConfigurations gets experiments from the service
func GenerateNewExperimentConfigurations() (*models.ExperimentConfigurations, error) {
	restClient, restCLientError := vampclientprovider.GetRestClient()
	if restCLientError != nil {
		return nil, errors.New("Rest Client can not be initiliazed")
	}
	values := make(map[string]string)
	values["project"] = vampclientprovider.Project
	values["cluster"] = vampclientprovider.Cluster
	values["virtual_cluster"] = vampclientprovider.VirtualCluster
	listOfExperiments, err := restClient.List("experiment", "json", values, false)
	if err != nil {
		return nil, err
	}
	experimentConfs, confReadError := ParseExperimentConfiguration(listOfExperiments)
	if confReadError != nil {
		return nil, confReadError
	}
	logging.Info("experimentConfs: %v\n", *experimentConfs)
	return experimentConfs, nil
}

func RefreshExperimentConfigurations() error {
	logging.Info("Refresh Experiment Configurations at: %v\n", time.Now())
	if atomic.LoadInt32(&activeConfigurationID) == 0 {
		experimentConfigurations, confError := GenerateNewExperimentConfigurations()
		if confError != nil {
			return confError
		}
		ExperimentConfigurations1 = *experimentConfigurations
		atomic.StoreInt32(&activeConfigurationID, 1)
		return nil
	} else {
		experimentConfigurations, confError := GenerateNewExperimentConfigurations()
		if confError != nil {
			return confError
		}
		ExperimentConfigurations0 = *experimentConfigurations
		atomic.StoreInt32(&activeConfigurationID, 0)
		return nil
	}
}

func SetupConfigurator() {
	logging.Info("SetupConfigurator at %v Refresh period: %v\n", time.Now(), RefreshPeriod)
	RefreshExperimentConfigurations()
	ticker := time.NewTicker(RefreshPeriod)
	go func() {
		for {
			select {
			case <-ticker.C:
				RefreshExperimentConfigurations()
			}
		}
	}()
}

func SendExperimentStats(experimentStatsGroup *models.ExperimentStatsGroup) error {
	restClient, restCLientError := vampclientprovider.GetRestClient()
	if restCLientError != nil {
		return errors.New("Rest Client can not be initiliazed")
	}
	values := make(map[string]string)
	values["project"] = vampclientprovider.Project
	values["cluster"] = vampclientprovider.Cluster
	values["virtual_cluster"] = vampclientprovider.VirtualCluster
	metricName := "target"

	for experimentName, experimentStat := range experimentStatsGroup.ExperimentStatsMap {
		for subsetName, subsetStat := range experimentStat.Subsets {
			metricValue := &clientmodels.MetricValue{
				Timestamp:         time.Now().Unix(),
				NumberOfElements:  int64(subsetStat.NumberOfElements),
				StandardDeviation: subsetStat.StandardDeviation,
				Average:           subsetStat.Average,
			}
			values["experiment"] = experimentName
			vars := strings.SplitN(subsetName, "/", 3)
			if len(vars) < 3 {
				logging.Error("Subset info name doesn't have enough knowlegde: %v\n", subsetName)
				continue
			}
			values["destination"] = vars[0]
			values["port"] = vars[1]
			values["subset"] = vars[2]

			// logging.Info("Sending Experiment metrics is not implemented yet: %v/%v => %v\n", experimentName, subsetName, metric)
			_, sendExperimentError := restClient.PushMetricValue(metricName, metricValue, values)
			if sendExperimentError != nil {
				logging.Error("SendExperimentError: %v\n", sendExperimentError)
			} else {
				logging.Info("Results sent.\n")
			}
		}
	}
	return nil
}

// SendMetricStats send metrics to vamp api
func SendMetricStats(metricName string,
	destination string,
	port string,
	subset string,
	experiment string,
	metricStats *models.MetricStats) error {
	restClient, restCLientError := vampclientprovider.GetRestClient()
	if restCLientError != nil {
		return errors.New("Rest Client can not be initiliazed")
	}
	values := make(map[string]string)
	values["project"] = vampclientprovider.Project
	values["cluster"] = vampclientprovider.Cluster
	values["virtual_cluster"] = vampclientprovider.VirtualCluster
	values["destination"] = destination
	values["experiment"] = experiment
	values["port"] = port
	values["subset"] = subset
	metricValue := &clientmodels.MetricValue{
		Timestamp:         time.Now().Unix(),
		NumberOfElements:  metricStats.NumberOfElements,
		StandardDeviation: metricStats.StandardDeviation,
		Average:           metricStats.Average,
		Sum:               metricStats.Average,
		Median:            metricStats.Median,
		Min:               metricStats.Min,
		Max:               metricStats.Max,
		Rate:              metricStats.Rate,
		P999:              metricStats.P999,
		P99:               metricStats.P99,
		P95:               metricStats.P95,
		P75:               metricStats.P75,
	}
	_, pushMetricValueError := restClient.PushMetricValue(metricName, metricValue, values)
	if pushMetricValueError != nil {
		logging.Error("Push Metric Value Error: %v\n", pushMetricValueError)
		return pushMetricValueError
	}
	logging.Info("Metric sent.\n")

	return nil
}
