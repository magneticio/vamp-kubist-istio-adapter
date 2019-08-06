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
	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/metriclogger"
	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/models"
	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/subsetmapper"
	"github.com/magneticio/vampkubistcli/logging"
)

const bufferSize = 1000

var LogInstanceChannel = make(chan *models.LogInstance, bufferSize)

var latencyMetricLoggerGroup *metriclogger.MetricLoggerGroup = metriclogger.NewMetricLoggerGroup("latency")

var ExperimentLoggers0 models.ExperimentLoggers
var ExperimentLoggers1 models.ExperimentLoggers

// this is added for testing
var SendExperimentLoggers = true

var activeLoggerID int32

const RefreshPeriod = 60 * time.Second

/*
Example of a real log instance:

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

func RunProcessor() {
	configurator.SetupConfigurator()
	subsetmapper.Setup()
	SetupProcessor()
	for {
		logInstance := <-LogInstanceChannel
		ProcessInstanceForMetrics(logInstance)
		ProcessInstanceForExperiments(configurator.GetExperimentConfigurations(), logInstance)
	}
}

// ProcessInstanceForMetrics processes a log instance for extracting metrics
func ProcessInstanceForMetrics(logInstance *models.LogInstance) {
	timestamp := logInstance.Timestamp
	destination := logInstance.Destination
	port := logInstance.DestinationPort
	labels := logInstance.DestinationLabels
	if latency, ok := logInstance.Values["latency"]; ok {
		value := ConvertToFloat64(latency)
		logging.Info("destination: %v port: %v labels: %v\n", destination, port, labels)
		subsets := subsetmapper.GetSubsetByLabels(destination, labels)
		logging.Info("subsets: %v\n", subsets)
		for _, subsetWithPorts := range subsets {
			for _, portWith := range subsetWithPorts.Ports {
				if string(portWith) == port {
					latencyMetricLoggerGroup.GetMetricLogger(destination, port, subsetWithPorts.Subset).Push(timestamp, value)
				}
			}
		}
	}
}

func GetMetricLoggers() error {
	return nil
}

func ProcessInstanceForExperiments(
	experimentConfigurations *models.ExperimentConfigurations,
	logInstance *models.LogInstance) {

	header := http.Header{}
	if cookies, ok := logInstance.Values["cookies"]; ok {
		if cookiesString, ok2 := cookies.(string); ok2 {
			header.Add("Cookie", cookiesString)
		} else {
			return // string conversion problem
		}
	} else {
		return // no cookie
	}
	request := http.Request{
		Header: header,
	}
	experimentLogger := GetExperimentLoggers()
	for _, cookie := range request.Cookies() {
		if experimentConf, ok := experimentConfigurations.ExperimentConfigurationMap[cookie.Name]; ok {
			logging.Info("Cookie: %v\n", cookie.Value)
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
					if targetRegex.MatchString(logInstance.URL) {
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

func GetRegexForStartsWithPath(targetPath string) (*regexp.Regexp, error) {
	regexString := "^" + regexp.QuoteMeta(targetPath)
	return regexp.Compile(regexString)
}

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

func GetExperimentLoggers() *models.ExperimentLoggers {
	if atomic.LoadInt32(&activeLoggerID) == 0 {
		return &ExperimentLoggers0
	}
	return &ExperimentLoggers1
}

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

func ProcessExperimentLoggers(experimentLoggers *models.ExperimentLoggers) error {
	if experimentLoggers == nil {
		return errors.New("experimentLoggers is nil")
	}
	experimentStatsGroup := &models.ExperimentStatsGroup{
		ExperimentStatsMap: make(map[string]models.ExperimentStats),
	}
	// experimentStatsMap := make(map[string]models.ExperimentStats)
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
	if SendExperimentLoggers {
		// send stats to the server
		return configurator.SendExperimentStats(experimentStatsGroup)
	} else {
		return nil
	}
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

func ConvertToFloat64(i interface{}) float64 {
	switch v := i.(type) {
	case int:
		return float64(v)
	case string:
		if s, err := strconv.ParseFloat(v, 64); err == nil {
			return s
		} else {
			fmt.Printf("Float parsing failed %v!\n", i)
		}
	case time.Duration:
		return float64(v.Nanoseconds()) / float64(1e6) // convert to milliseconds
	default:
		fmt.Printf("unknown type type %T!\n", v)
	}
	return 0
}
