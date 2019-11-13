package processor

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/configurator"
	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/healthmetrics"
	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/k8smetrics"
	metriclogger "github.com/magneticio/vamp-kubist-istio-adapter/adapter/metriclogger"
	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/models"
	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/subsetmapper"
	"github.com/magneticio/vampkubistcli/logging"
)

// bufferSize for processing queue, it should be incremented to handle burst of input
const bufferSize = 1000

// LogInstanceChannel is the main channel to store logInstances to be processed
var LogInstanceChannel = make(chan *models.LogInstance, bufferSize)

// processSubsetMapper is the subsetmapper used for processor
var processSubsetMapper = subsetmapper.New()

// TODO: Experiment loggers should be merged to metric loggers or refactored

// ExperimentLoggers0 is the first logger for experiments
var ExperimentLoggers0 models.ExperimentLoggers

// ExperimentLoggers1 is the first logger for experiments
var ExperimentLoggers1 models.ExperimentLoggers

// SendExperimentLoggers is added for testing (false for testing)
var SendExperimentLoggers = true

// activeLoggerID sets active experiment logger
var activeLoggerID int32

// RefreshPeriod is the period used for refreshing experiment logging period
const RefreshPeriod = 60 * time.Second

/*
TODO: move this example to a proper place

Example of a real log instance that is provided for istio

TimeStamp:  2019-06-19T12:09:30.732103777Z
Severity:  info
url :  /cart?variant_id=1
user :  unknown
cookies :  ex-1_user=3c445470-721f-48d2-ad4a-02de5d19988c;ex-1=dest-1-9191-subset2;guest_token=ImF3SzNsTlMyaHVLLUVGT3lqZXFjRFExNTYwOTQ0ODY2MTI4Ig%3D%3D--327ef99d0408d7d2738b87379d80015
a760d8594;_vshop_session=cURvN3QwQnFicy9JWWZWNWkzcy9vWDBzd09WN3RHTkZsWUs0eEwvblIwdWxwMEVjODlOVGhGbGlNNFN0US9ZV3hDRkFBTTNic0cwMnZuSVdtNS9ZaVE4Z29neGpkbFIxNVV5OG4xU3pvN3RxR3JQSHNGQ2Z2NFc

yQ2lzUFdYc1d5RFpEQTVlemJmenQ4NUhWWE1tV1B2MDdkc0N4RlNpQjZ5eEJ0Y3l5VWlkREIvby9JV1N0cms4a1RzbnV6Z21sLS1Mc1lEdHJTaEk1UDQrNnJSdWhmT0l3PT0%3D--35e006358222b813f496eb7e3ab4136886f1d0d0
destination :  demo-app
latency :  36.749194ms
responseCode :  200
responseSize :  4405
source :  gw-1-gateway
*/

// SetupProcessor sets up a regular process to refresh and send experiment metrics
func SetupProcessor() {
	logging.Info("SetupProcessor at %v Refresh period: %v\n", time.Now(), RefreshPeriod)
	RefreshExperimentLoggers()
	ticker := time.NewTicker(RefreshPeriod)
	go func() {
		for {
			select {
			case <-ticker.C:
				RefreshExperimentLoggers()
			}
		}
	}()
}

// RunProcessor is the main function for Processor related tasks
func RunProcessor() {
	configurator.SetupConfigurator()
	processSubsetMapper.Setup()
	metriclogger.Setup()
	k8smetrics.Setup(LogInstanceChannel)
	healthmetrics.Setup(LogInstanceChannel)
	SetupProcessor()
	for {
		logInstance := <-LogInstanceChannel
		ProcessInstanceForMetrics(logInstance)
		ProcessInstanceForExperiments(configurator.GetExperimentConfigurations(), logInstance)
	}
}

// GetStringFromInterface is a helper method to convert interface to string where possible
func GetStringFromInterface(values map[string]interface{}, key string) string {
	if value, ok := values[key]; ok {
		return fmt.Sprintf("%v", value)
	}
	return ""
}

// ProcessInstanceForMetrics processes a log instance for extracting metrics
func ProcessInstanceForMetrics(logInstance *models.LogInstance) {
	timestamp := logInstance.Timestamp
	destination := logInstance.Destination
	port := logInstance.DestinationPort
	labels := logInstance.DestinationLabels
	subsets := processSubsetMapper.GetSubsetByLabels(destination, labels)
	// TODO: add error check
	apiProtocol := GetStringFromInterface(logInstance.Values, "apiProtocol")
	requestMethod := GetStringFromInterface(logInstance.Values, "requestMethod")
	responseCode := GetStringFromInterface(logInstance.Values, "responseCode")

	// logging.Info("Instance: %v %v %v %v\n", logInstance, apiProtocol, requestMethod, responseCode)
	for metricName, metricValue := range logInstance.Values {
		if metricInfo, existInMetricDefinitions := metriclogger.MetricDefinitions[metricName]; existInMetricDefinitions {
			groupNames := metricInfo.GetMetricLoggerNames(metricName, apiProtocol, requestMethod, responseCode, metricValue)
			// logging.Info("Group Names for %v, %v, %v, %v, %v\n", metricName, apiProtocol, requestMethod, responseCode, metricValue)
			for _, groupName := range groupNames {
				metricLoggerGroup := metriclogger.GetOrCreateMetricLoggerGroup(groupName, metricInfo.Type)
				for _, subsetInfo := range subsets {
					// logging.Info("Subset info: %v\n", subsetInfo)
					if len(subsetInfo.SubsetWithPorts.Ports) > 0 {
						for _, destWithPorts := range subsetInfo.SubsetWithPorts.Ports {
							if port != "" {
								// Service port and service target port should be handled.
								// LogInstance has one destination port where destination is a container
								// port is expected to be the target port not the service port
								// A map of service port to target port mapping
								if strconv.Itoa(destWithPorts.TargetPort) == port {
									metricLoggerGroup.GetMetricLogger(subsetInfo.DestinationName, port, subsetInfo.SubsetWithPorts.Subset).Push(timestamp, metricValue)
								} else {
									// TODO: test with different service port and container port case
									// Read the comments above
									logging.Info("Ports: %v - %v\n", destWithPorts.Port, port)
									metricLoggerGroup.GetMetricLogger(subsetInfo.DestinationName, strconv.Itoa(destWithPorts.Port), subsetInfo.SubsetWithPorts.Subset).Push(timestamp, metricValue)
								}
							} else {
								metricLoggerGroup.GetMetricLogger(subsetInfo.DestinationName, port, subsetInfo.SubsetWithPorts.Subset).Push(timestamp, metricValue)
							}
						}
					} else {
						metricLoggerGroup.GetMetricLogger(subsetInfo.DestinationName, port, subsetInfo.SubsetWithPorts.Subset).Push(timestamp, metricValue)
					}
				}
			}
		}
	}
}

// ProcessInstanceForExperiments extract experiment metrics if url and cookie is provided
func ProcessInstanceForExperiments(
	experimentConfigurations *models.ExperimentConfigurations,
	logInstance *models.LogInstance) {
	URL := ""
	if url, ok := logInstance.Values["url"]; ok {
		if urlString, ok2 := url.(string); ok2 {
			URL = urlString
		} else {
			logging.Error("URL conversion to string failed\n")
			return // string conversion problem
		}
	} else {
		// logging.Error("No url in the instance\n")
		return // no url
	}
	header := http.Header{}
	if cookies, ok := logInstance.Values["cookies"]; ok {
		if cookiesString, ok2 := cookies.(string); ok2 {
			header.Add("Cookie", cookiesString)
		} else {
			logging.Error("Cookie conversion to string failed\n")
			return // string conversion problem
		}
	} else {
		// logging.Error("No cookie in the instance\n")
		return // no cookie
	}
	// TODO: url can be added to request like cookies
	request := http.Request{
		Header: header,
	}
	experimentLogger := GetExperimentLoggers()
	for _, cookie := range request.Cookies() {
		// logging.Info("Cookie: %v\n", cookie.Value)
		if experimentConf, ok := experimentConfigurations.ExperimentConfigurationMap[cookie.Name]; ok {
			experimentName := cookie.Name
			if targetPath, ok2 := experimentConf.Subsets[cookie.Value]; ok2 {
				subsetName := cookie.Value
				userCookieName := experimentName + "_user"
				if userCookie, cookieErr := request.Cookie(userCookieName); cookieErr == nil {
					userID := userCookie.Value
					CreateEntrySafe(experimentLogger, experimentName, subsetName, userID)
					targetRegex, targetRegexError := GetRegexForStartsWithPath(targetPath)
					if targetRegexError != nil {
						logging.Info("Target Regex Error: %v\n", targetRegexError)
					}
					if targetRegex.MatchString(URL) {
						experimentLogger.ExperimentLogs[experimentName].SubsetLogs[subsetName].UserLogs[userID]++
					}
				} else {
					logging.Error("cookieErr: %v\n", cookieErr)
				}
			}
			break
		}
	}
}

// GetRegexForStartsWithPath creates a starts regex from any string
func GetRegexForStartsWithPath(targetPath string) (*regexp.Regexp, error) {
	regexString := "^" + regexp.QuoteMeta(targetPath)
	return regexp.Compile(regexString)
}

// CreateEntrySafe creates entries in experiment logger map by checking existance
func CreateEntrySafe(experimentLogger *models.ExperimentLoggers, experimentName, subsetName, userID string) {
	if experimentLogger.ExperimentLogs == nil {
		experimentLogger.ExperimentLogs = make(map[string]models.ExperimentLogs)
	}
	if _, ok := experimentLogger.ExperimentLogs[experimentName]; !ok {
		experimentLogger.ExperimentLogs[experimentName] = models.ExperimentLogs{
			SubsetLogs: map[string]models.SubsetLogs{
				subsetName: models.SubsetLogs{
					UserLogs: map[string]int{userID: 0},
				},
			},
		}
	}
	if _, ok := experimentLogger.ExperimentLogs[experimentName].SubsetLogs[subsetName]; !ok {
		experimentLogger.ExperimentLogs[experimentName].SubsetLogs[subsetName] =
			models.SubsetLogs{
				UserLogs: map[string]int{userID: 0},
			}
	}
	if _, ok := experimentLogger.ExperimentLogs[experimentName].SubsetLogs[subsetName].UserLogs[userID]; !ok {
		experimentLogger.ExperimentLogs[experimentName].SubsetLogs[subsetName].UserLogs[userID] = 0
	}
}

// GetExperimentLoggers returns active logger
func GetExperimentLoggers() *models.ExperimentLoggers {
	if atomic.LoadInt32(&activeLoggerID) == 0 {
		return &ExperimentLoggers0
	}
	return &ExperimentLoggers1
}

// RefreshExperimentLoggers triggers a processing and then refresing of active logger
func RefreshExperimentLoggers() error {
	logging.Info("Process and Clean Experiment Loggers at: %v\n", time.Now())
	if atomic.LoadInt32(&activeLoggerID) == 0 {
		atomic.StoreInt32(&activeLoggerID, 1)
		processError := ProcessExperimentLoggers(&ExperimentLoggers1)
		if processError != nil {
			return processError
		}
		// clear
		ExperimentLoggers1 = models.ExperimentLoggers{}
		return nil
	} else {
		processError := ProcessExperimentLoggers(&ExperimentLoggers0)
		if processError != nil {
			return processError
		}
		// clean
		ExperimentLoggers0 = models.ExperimentLoggers{}
		atomic.StoreInt32(&activeLoggerID, 0)
		return nil
	}
}

// ProcessExperimentLoggers processes a logger and calculates experiment stats
func ProcessExperimentLoggers(experimentLoggers *models.ExperimentLoggers) error {
	if experimentLoggers == nil {
		return errors.New("experimentLoggers is nil")
	}
	experimentStatsGroup := &models.ExperimentStatsGroup{
		ExperimentStatsMap: make(map[string]models.ExperimentStats),
	}

	experimentConfigurations := configurator.GetExperimentConfigurations()
	for experimentName, experimentConf := range experimentConfigurations.ExperimentConfigurationMap {
		if _, ok := experimentLoggers.ExperimentLogs[experimentName]; !ok {
			logging.Info("Experiment doesn't exist %v\n", experimentName)
			continue
		}
		experimentStatsGroup.ExperimentStatsMap[experimentName] = models.ExperimentStats{
			Subsets: make(map[string]models.SubsetStats),
		}
		for subsetName := range experimentConf.Subsets {
			if _, ok2 := experimentLoggers.ExperimentLogs[experimentName].SubsetLogs[subsetName]; !ok2 {
				logging.Info("SubsetName doesn't exist %v\n", subsetName)
				continue
			}
			n := float64(len(experimentLoggers.ExperimentLogs[experimentName].SubsetLogs[subsetName].UserLogs))
			if n == 0 {
				logging.Info("No data for SubsetName: %v\n", subsetName)
				continue
			}
			experimentStatsGroup.ExperimentStatsMap[experimentName].Subsets[subsetName] = models.SubsetStats{
				NumberOfElements:  n,
				Average:           0,
				StandardDeviation: 0,
			}
			var average float64 = 0
			for _, count := range experimentLoggers.ExperimentLogs[experimentName].SubsetLogs[subsetName].UserLogs {
				average += float64(count) / n
			}
			experimentStatsGroup.ExperimentStatsMap[experimentName].Subsets[subsetName] = models.SubsetStats{
				NumberOfElements:  n,
				Average:           average,
				StandardDeviation: 0,
			}
			if n < 1 {
				logging.Info("Not enough data for calculation of standard deviation n=%v\n", n)
				continue
			}
			var differentiationSum float64 = 0
			for _, count := range experimentLoggers.ExperimentLogs[experimentName].SubsetLogs[subsetName].UserLogs {
				differentiationSum += math.Pow(float64(count)-average, 2) / n
			}

			standardDeviation := math.Sqrt(differentiationSum)
			experimentStatsGroup.ExperimentStatsMap[experimentName].Subsets[subsetName] = models.SubsetStats{
				NumberOfElements:  n,
				Average:           average,
				StandardDeviation: standardDeviation,
			}
		}
	}
	logging.Info("experimentStatsMap: %v\n", experimentStatsGroup.ExperimentStatsMap)
	if SendExperimentLoggers { // This is only false when testing
		// send stats to the server
		return configurator.SendExperimentStats(experimentStatsGroup)
	}
	return nil
}

// GetMergedExperimentLoggers is added for testing
func GetMergedExperimentLoggers() *models.ExperimentLoggers {
	experimentConfigurations := configurator.GetExperimentConfigurations()
	merged := ExperimentLoggers0
	for experimentName, experimentConf := range experimentConfigurations.ExperimentConfigurationMap {
		for subsetName := range experimentConf.Subsets {
			for userID, count := range ExperimentLoggers1.ExperimentLogs[experimentName].SubsetLogs[subsetName].UserLogs {
				CreateEntrySafe(&merged, experimentName, subsetName, userID)
				merged.ExperimentLogs[experimentName].SubsetLogs[subsetName].UserLogs[userID] += count
			}
		}
	}
	return &merged
}
