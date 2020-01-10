package metriclogger

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	types "github.com/gogo/protobuf/types"
	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/configurator"
	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/models"
	"github.com/magneticio/vampkubistcli/logging"
	"github.com/montanaflynn/stats"
)

// DefaultRefreshPeriod represents the bucket move period
const DefaultRefreshPeriod = 30 * time.Second

// DefaultMetricDuration will be used for calculation of metrics
const DefaultMetricDuration = 30 * time.Second

// Number Of Buckets that can be used for metric storage
const DefaultNumberOfBuckets = 3

// MetricDuration < ( NumberOfBuckets -1 ) * RefreshPeriod

// MetricDefinitions is a mapping of metrics from input name to output name
var MetricDefinitions = map[string]MetricInfo{
	"responseDuration":    NewMetricInfo(Valued, "response_duration_%v"),
	"responseCode":        NewMetricInfo(Categorical, "response_code_%v"),
	"Memory":              NewMetricInfo(Valued, "memory_%v"),
	"CPU":                 NewMetricInfo(Valued, "cpu_%v"),
	"Replicas":            NewMetricInfo(Valued, "replicas_%v"),
	"UpdatedReplicas":     NewMetricInfo(Valued, "updated_replicas_%v"),
	"ReadyReplicas":       NewMetricInfo(Valued, "updated_replicas_%v"),
	"AvailableReplicas":   NewMetricInfo(Valued, "available_replicas_%v"),
	"UnavailableReplicas": NewMetricInfo(Valued, "unavailable_replicas_%v"),
	"Availability":        NewMetricInfo(Valued, "availability_%v"),
	"requests":            NewMetricInfo(Categorical, "requests_%v"),
}

// NewMetricInfo creates a metric info with given inputs
func NewMetricInfo(metricType MetricType, nameFormat string) MetricInfo {
	return MetricInfo{
		Type:       metricType,
		NameFormat: nameFormat,
	}
}

// MapValueToPossibleCodes generates codes for possible variations of a metric
func MapValueToPossibleCodes(apiProtocol string, requestMethod string, responseCode string) []string {
	// Switch by protocol to keep it open to develop for other protocol, like grpc and tcp
	apiProtocol = strings.ToLower(apiProtocol)
	requestMethod = strings.ToLower(requestMethod)
	codeString := fmt.Sprintf("%v", responseCode) // this can be int

	if apiProtocol == "" && requestMethod != "" {
		methods := map[string]bool{
			"get":    true,
			"put":    true,
			"post":   true,
			"head":   true,
			"delete": true,
			"option": true,
			"patch":  true,
		}
		if methods[requestMethod] {
			apiProtocol = "http"
		}
	}
	switch apiProtocol {
	case "http", "https":
		if len(codeString) > 0 {
			return []string{
				"",
				codeString,
				fmt.Sprintf("%vxx", string(codeString[0])),
				fmt.Sprintf("%v_%v", requestMethod, codeString),
				fmt.Sprintf("%v_%vxx", requestMethod, string(codeString[0])),
			}
		}
	default:
		return []string{""}
	}
	return []string{""}
}

// MetricLoggerGroupMap returns map of metric logger groups
// This is the default map other new values will be created on the fly
var MetricLoggerGroupMap = map[string]*MetricLoggerGroup{
	"response_duration":         NewMetricLoggerGroup("response_duration", Valued),
	"response_duration_200":     NewMetricLoggerGroup("response_duration_200", Valued),
	"response_duration_2xx":     NewMetricLoggerGroup("response_duration_2xx", Valued),
	"response_duration_get_2xx": NewMetricLoggerGroup("response_duration_get_2xx", Valued),
	"response_duration_500":     NewMetricLoggerGroup("response_duration_500", Valued),
	"response_duration_5xx":     NewMetricLoggerGroup("response_duration_5xx", Valued),
	"response_duration_get_5xx": NewMetricLoggerGroup("response_duration_get_5xx", Valued),
	"response_code_200":         NewMetricLoggerGroup("response_code_200", Categorical),
	"response_code_2xx":         NewMetricLoggerGroup("response_code_2xx", Categorical),
	"response_code_get_2xx":     NewMetricLoggerGroup("response_code_get_2xx", Categorical),
	"response_code_500":         NewMetricLoggerGroup("response_code_500", Categorical),
	"response_code_5xx":         NewMetricLoggerGroup("response_code_5xx", Categorical),
	"memory":                    NewMetricLoggerGroup("memory", Valued),
	"cpu":                       NewMetricLoggerGroup("cpu", Valued),
	"replicas":                  NewMetricLoggerGroup("replicas", Valued),
	"updated_replicas":          NewMetricLoggerGroup("updated_replicas", Valued),
	"ready_replicas":            NewMetricLoggerGroup("ready_replicas", Valued),
	"available_replicas":        NewMetricLoggerGroup("available_replicas", Valued),
	"unavailable_replicas":      NewMetricLoggerGroup("unavailable_replicas", Valued),
	"availability":              NewMetricLoggerGroup("availability", Valued),
}

// GetOrCreateMetricLoggerGroup adds non existing metrics to the metric list and returns it
func GetOrCreateMetricLoggerGroup(name string, metricType MetricType) *MetricLoggerGroup {
	if _, ok := MetricLoggerGroupMap[name]; !ok {
		MetricLoggerGroupMap[name] = NewMetricLoggerGroup(name, metricType)
		MetricLoggerGroupMap[name].Setup()
	}
	return MetricLoggerGroupMap[name]
}

// MetricType represent enum type of Valued or Categorical metrics
type MetricType int

const (
	// Valued metrics that are not time series
	Valued MetricType = iota
	// Categorical metrics are time series like 200s, 500s per second
	Categorical
)

func (d MetricType) String() string {
	return [...]string{"Valued", "Categorical"}[d]
}

// MetricInfo is a struct used for defining metric name mapping
type MetricInfo struct {
	Type       MetricType
	NameFormat string
}

// MetricLoggerGroup is used to group similar metrics to decrease processing
type MetricLoggerGroup struct {
	Name          string
	MetricType    MetricType
	RefreshPeriod time.Duration
	MetricLoggers map[string]*MetricLogger
	IsSetup       bool
}

// MetricLogger is a struct that holds metrics for a single stream of metric
type MetricLogger struct {
	Name            string
	MetricType      MetricType
	Destination     string
	Port            string
	Subset          string
	NumberOfBuckets int
	ValueMaps       []map[int64][]float64
	ActiveID        int32
	RefreshPeriod   time.Duration
	LastUpdate      int64
}

// MetricValues is a struct to hold one bucket of metric stream
type MetricValues struct {
	StartTime int64
	EndTime   int64
	Values    map[int64][]float64
}

// Setup sets up timer to regularly process a metric group.
// This is actually not needed anymore, since dynamic setup is possible now.
func Setup() {
	logging.Info("Setting up Metric Logger Groups")
	for _, val := range MetricLoggerGroupMap {
		val.Setup()
	}
}

// NewMetricLoggerGroup creates a new metric logger group
func NewMetricLoggerGroup(metricName string, metricType MetricType) *MetricLoggerGroup {
	metricLoggerGroup := &MetricLoggerGroup{
		Name:          metricName,
		MetricType:    metricType,
		MetricLoggers: make(map[string]*MetricLogger),
		RefreshPeriod: DefaultRefreshPeriod,
		IsSetup:       false,
	}
	return metricLoggerGroup
}

// GetMetricLogger gets or creates a metric logger for destination, port, subset
func (g *MetricLoggerGroup) GetMetricLogger(destination string, port string, subset string) *MetricLogger {
	key := fmt.Sprintf("%v:%v/%v", destination, port, subset)
	if _, ok := g.MetricLoggers[key]; !ok {
		g.MetricLoggers[key] = NewMetricLogger(destination, port, subset, g.Name, g.MetricType, DefaultRefreshPeriod)
	}
	return g.MetricLoggers[key]
}

// NewMetricLogger creates a new metric logger and pre-initializes buckets
func NewMetricLogger(destination string, port string, subset string, metricName string, metricType MetricType, refreshPeriod time.Duration) *MetricLogger {
	metricLogger := &MetricLogger{
		Name:            metricName,
		MetricType:      metricType,
		Destination:     destination,
		Port:            port,
		Subset:          subset,
		RefreshPeriod:   refreshPeriod,
		NumberOfBuckets: DefaultNumberOfBuckets,
		ValueMaps:       []map[int64][]float64{},
	}
	// Important Note: MetricDuration < ( NumberOfBuckets -1 ) * RefreshPeriod
	// Pre initilize value maps
	for i := 0; i < metricLogger.NumberOfBuckets; i++ {
		metricLogger.ValueMaps = append(metricLogger.ValueMaps, make(map[int64][]float64, 0))
	}
	atomic.StoreInt32(&metricLogger.ActiveID, 0)
	return metricLogger
}

// Setup sets up a periodic process to calculate and send metrics
func (g *MetricLoggerGroup) Setup() {
	if g.IsSetup == true {
		return
	}
	g.IsSetup = true
	logging.Info("Setup Metric logger Group for %v Refresh period: %v\n", g.Name, g.RefreshPeriod)
	for _, metricLogger := range g.MetricLoggers {
		metricLogger.RefreshMetricLogger()
	}
	ticker := time.NewTicker(g.RefreshPeriod)
	go func() {
		for {
			select {
			case <-ticker.C:
				currentTime := time.Now().Unix()
				for name, metricLogger := range g.MetricLoggers {
					if metricLogger.LastUpdate < currentTime-int64(metricLogger.RefreshPeriod.Seconds())*int64(metricLogger.NumberOfBuckets) {
						delete(g.MetricLoggers, name)
					}
				}
				for _, metricLogger := range g.MetricLoggers {
					metricLogger.RefreshMetricLogger()
				}
			}
		}
	}()
}

// ConvertToFloat64 gets an interface and converts into float64
// All metrics are stored as float64, if a new type added this should be updated
func ConvertToFloat64(i interface{}) float64 {
	switch v := i.(type) {
	case int:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case float32:
		return float64(v)
	case float64:
		return v
	case string:
		if s, err := strconv.ParseFloat(v, 64); err == nil {
			return s
		} else {
			fmt.Printf("Float parsing failed %v!\n", i)
		}
	case time.Duration:
		return float64(v.Nanoseconds()) / float64(1e6) // convert to milliseconds
	case types.Duration:
		return float64(v.Seconds)*float64(1e3) + float64(v.Nanos)/float64(1e6) // convert to milliseconds
	case *types.Duration:
		return float64(v.Seconds)*float64(1e3) + float64(v.Nanos)/float64(1e6) // convert to milliseconds
	default:
		fmt.Printf("unknown type type %T!\n", v)
	}
	return 0
}

// GetMetricLoggerNames return name to be used as metric identifier
func (m *MetricInfo) GetMetricLoggerNames(name string, apiProtocol, requestMethod, responseCode string, value interface{}) []string {
	switch m.Type {
	case Categorical:
		codes := MapValueToPossibleCodes(apiProtocol, requestMethod, responseCode)
		names := make([]string, 0, len(codes))
		for _, code := range codes {
			groupName := strings.TrimSuffix(fmt.Sprintf(m.NameFormat, code), "_")
			names = append(names, groupName)
		}
		return names
	case Valued:
		codes := MapValueToPossibleCodes(apiProtocol, requestMethod, responseCode)
		names := make([]string, 0, len(codes))
		for _, code := range codes {
			groupName := strings.TrimSuffix(fmt.Sprintf(m.NameFormat, code), "_")
			names = append(names, groupName)
		}
		return names
	default: // default is valued
		return []string{name}
	}
}

// Push add data to the active bucket
func (m *MetricLogger) Push(timestamp int64, value interface{}) {
	id := atomic.LoadInt32(&m.ActiveID)
	m.LastUpdate = timestamp
	if _, ok := m.ValueMaps[id][timestamp]; !ok {
		switch m.MetricType {
		case Categorical:
			m.ValueMaps[id][timestamp] = []float64{1.0}
		case Valued:
			floatValue := ConvertToFloat64(value)
			m.ValueMaps[id][timestamp] = []float64{floatValue}
		default: // default is valued
			floatValue := ConvertToFloat64(value)
			m.ValueMaps[id][timestamp] = []float64{floatValue}
		}
		// add first value and return
		return
	}
	switch m.MetricType {
	case Categorical:
		m.ValueMaps[id][timestamp][0] += 1.0
	case Valued:
		floatValue := ConvertToFloat64(value)
		m.ValueMaps[id][timestamp] = append(m.ValueMaps[id][timestamp], floatValue)
	default: // default is valued
		floatValue := ConvertToFloat64(value)
		m.ValueMaps[id][timestamp] = append(m.ValueMaps[id][timestamp], floatValue)
	}
}

// MergeValuesOfNonActiveBucketsWithTimeBasedFiltering merges values in unused buckets
func (m *MetricLogger) MergeValuesOfNonActiveBucketsWithTimeBasedFiltering(now int64) *MetricValues {
	id := atomic.LoadInt32(&m.ActiveID)
	timeStart := now - int64(m.RefreshPeriod.Seconds())
	mergedValueMap := make(map[int64][]float64, 0)
	for index, valueMap := range m.ValueMaps {
		if int32(index) != id {
			for timestamp, value := range valueMap {
				if timestamp >= timeStart {
					if _, ok := mergedValueMap[timestamp]; !ok {
						mergedValueMap[timestamp] = value
					} else {
						mergedValueMap[timestamp] = append(mergedValueMap[timestamp], value...)
					}
				}
			}
		}
	}
	return &MetricValues{
		StartTime: timeStart,
		EndTime:   now,
		Values:    mergedValueMap,
	}
}

// RefreshMetricLogger trigger process and cleanup of metric buckets
func (m *MetricLogger) RefreshMetricLogger() error {
	logging.Info("Process and Clean Metriclogger Values for %v\n", m.Name)
	id := atomic.LoadInt32(&m.ActiveID)
	// TODO: review this logic of calculating next active id
	nextID := (int(id) + 1) % len(m.ValueMaps)
	m.ValueMaps[nextID] = make(map[int64][]float64, 0)
	now := time.Now().Unix()
	atomic.StoreInt32(&m.ActiveID, int32(nextID))
	metricValues := m.MergeValuesOfNonActiveBucketsWithTimeBasedFiltering(now)
	processError := m.ProcessMetricLogger(metricValues)
	if processError != nil {
		logging.Error("Error in Refresh Metric Logger: %v\n", processError)
		return processError
	}
	return nil
}

// ProcessMetricLogger processes metrics and trigger send metrics
func (m *MetricLogger) ProcessMetricLogger(metricValues *MetricValues) error {
	switch m.MetricType {
	case Categorical:
		return m.ProcessCategoricalMetricLogger(metricValues)
	case Valued:
		return m.ProcessValuedMetricLogger(metricValues)
	default: // default is valued
		return m.ProcessValuedMetricLogger(metricValues)
	}
}

// ProcessValuedMetricLogger processes metrics and trigger send metrics
func (m *MetricLogger) ProcessValuedMetricLogger(metricValues *MetricValues) error {
	allValues := make([]float64, 0, len(metricValues.Values))
	for _, v := range metricValues.Values {
		allValues = append(allValues, v...)
	}
	metricStats, metricStatsError := CalculateMetricStats(allValues)
	if metricStatsError != nil {
		return metricStatsError
	}
	logging.Info("Metrics should be sent now: %v => %v\n", m.Name, metricStats)
	// go sendMetric( ... )

	sendMetricStatsError := configurator.SendMetricStats(
		m.Name,
		m.Destination,
		m.Port,
		m.Subset,
		"", // no experiment here
		metricStats)
	if sendMetricStatsError != nil {
		return sendMetricStatsError
	}
	return nil
}

// ProcessCategoricalMetricLogger processes metrics and trigger send metrics
func (m *MetricLogger) ProcessCategoricalMetricLogger(metricValues *MetricValues) error {
	allValues := make([]float64, 0, len(metricValues.Values))
	timeLength := metricValues.EndTime - metricValues.StartTime + 1
	timeSeriesAsAvg := make([]float64, timeLength, timeLength)
	for t, v := range metricValues.Values {
		allValues = append(allValues, v...)
		avg, _ := stats.Mean(v)
		index := t - metricValues.StartTime
		if index >= 0 && index < timeLength {
			timeSeriesAsAvg[index] = avg
		}
	}
	metricStats, metricStatsError := CalculateMetricStats(timeSeriesAsAvg)
	if metricStatsError != nil {
		return metricStatsError
	}

	logging.Info("Metrics should be sent now: %v => %v\n", m.Name, metricStats)
	// go sendMetric( ... )
	sendMetricStatsError := configurator.SendMetricStats(
		m.Name,
		m.Destination,
		m.Port,
		m.Subset,
		"", // no experiment here
		metricStats)
	if sendMetricStatsError != nil {
		return sendMetricStatsError
	}

	return nil
}

// CalculateMetricStats calculate metris stats with stats library
func CalculateMetricStats(valuesRaw []float64) (*models.MetricStats, error) {

	values := stats.LoadRawData(valuesRaw)
	// NumberOfElements
	metricStats := &models.MetricStats{
		NumberOfElements: int64(values.Len()),
	}

	if values.Len() == 0 {
		return metricStats, nil
	}

	// Average
	if calculation, calculationErr := values.Mean(); calculationErr == nil {
		metricStats.Average = calculation
	} else {
		logging.Error("Calculation Error: %v\n", calculationErr)
	}

	// StandardDeviation
	if calculation, calculationErr := values.StandardDeviation(); calculationErr == nil {
		metricStats.StandardDeviation = calculation
	} else {
		logging.Error("Calculation Error: %v\n", calculationErr)
	}

	// Sum
	if calculation, calculationErr := values.Sum(); calculationErr == nil {
		metricStats.Sum = calculation
	} else {
		logging.Error("Calculation Error: %v\n", calculationErr)
	}

	if metricStats.Sum == 0 {
		return metricStats, nil
	}
	// TODO: Add more checks

	// Median
	if calculation, calculationErr := values.Median(); calculationErr == nil {
		metricStats.Median = calculation
	} else {
		logging.Error("Calculation Error: %v\n", calculationErr)
	}

	// Min
	if calculation, calculationErr := stats.Min(values); calculationErr == nil {
		metricStats.Min = calculation
	} else {
		logging.Error("Calculation Error: %v\n", calculationErr)
	}

	// Max
	if calculation, calculationErr := stats.Max(values); calculationErr == nil {
		metricStats.Max = calculation
	} else {
		logging.Error("Calculation Error: %v\n", calculationErr)
	}

	// P999
	if calculation, calculationErr := stats.Percentile(values, 0.999); calculationErr == nil {
		metricStats.P999 = calculation
	}

	// P99
	if calculation, calculationErr := stats.Percentile(values, 0.99); calculationErr == nil {
		metricStats.P99 = calculation
	}

	// P95
	if calculation, calculationErr := stats.Percentile(values, 0.95); calculationErr == nil {
		metricStats.P95 = calculation
	}

	// P75
	if calculation, calculationErr := stats.Percentile(values, 0.75); calculationErr == nil {
		metricStats.P75 = calculation
	}

	if values.Len() <= 1 {
		return metricStats, nil
	}

	// Rate
	if calculation, calculationErr := Rate(values); calculationErr == nil {
		metricStats.Rate = calculation
	} else {
		logging.Error("Calculation Error: %v\n", calculationErr)
	}

	return metricStats, nil
}

// Rate provides calculation of the rate
func Rate(input stats.Float64Data) (float64, error) {
	var result float64
	if input.Len() <= 1 {
		return math.NaN(),
			errors.New("Calculation of rate length requires array length to be larger than 1")
	}
	prev := input.Get(0)
	sum := float64(0)
	for i := 1; i < input.Len(); i++ {
		v := input.Get(i) - prev
		sum += v
		prev = input.Get(i)
	}
	result = sum / float64(input.Len()-1)
	return result, nil
}
