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

const DefaultRefreshPeriod = 30 * time.Second

var MetricDefinitions = map[string]MetricInfo{
	"responseDuration": MetricInfo{
		Type:       Valued,
		NameFormat: "response_duration_%v",
	},
	"responseCode": MetricInfo{
		Type:       Categorical,
		NameFormat: "response_code_%v",
	},
	"Memory": MetricInfo{
		Type:       Valued,
		NameFormat: "memory_%v",
	},
	"CPU": MetricInfo{
		Type:       Valued,
		NameFormat: "cpu_%v",
	},
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

// TODO: Populate this map automatically

// MetricLoggerGroupMap returns map of metric logger groups
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

type MetricInfo struct {
	Type       MetricType //todo enum
	NameFormat string
}

type MetricLoggerGroup struct {
	Name          string
	MetricType    MetricType
	RefreshPeriod time.Duration
	MetricLoggers map[string]*MetricLogger
}

type MetricLogger struct {
	Name          string
	MetricType    MetricType
	Destination   string
	Port          string
	Subset        string
	ValueMaps     []map[int64][]float64
	ActiveID      int32
	RefreshPeriod time.Duration
}

type MetricValue struct {
	Timestamp int64
	Value     float64
}

type MetricValues struct {
	StartTime int64
	EndTime   int64
	Values    map[int64][]float64
}

func Setup() {
	logging.Info("Setting up Metric Logger Groups")
	for _, val := range MetricLoggerGroupMap {
		val.Setup()
	}
}

func NewMetricLoggerGroup(metricName string, metricType MetricType) *MetricLoggerGroup {
	metricLoggerGroup := &MetricLoggerGroup{
		Name:          metricName,
		MetricType:    metricType,
		MetricLoggers: make(map[string]*MetricLogger),
		RefreshPeriod: DefaultRefreshPeriod,
	}
	// metricLoggerGroup.Setup()
	return metricLoggerGroup
}

func (g *MetricLoggerGroup) GetMetricLogger(destination string, port string, subset string) *MetricLogger {
	key := fmt.Sprintf("%v:%v/%v", destination, port, subset)
	if _, ok := g.MetricLoggers[key]; !ok {
		g.MetricLoggers[key] = NewMetricLogger(destination, port, subset, g.Name, g.MetricType, DefaultRefreshPeriod)
	}
	return g.MetricLoggers[key]
}

func NewMetricLogger(destination string, port string, subset string, metricName string, metricType MetricType, refreshPeriod time.Duration) *MetricLogger {
	metricLogger := &MetricLogger{
		Name:          metricName,
		MetricType:    metricType,
		Destination:   destination,
		Port:          port,
		Subset:        subset,
		RefreshPeriod: refreshPeriod,
		ValueMaps: []map[int64][]float64{
			make(map[int64][]float64, 0),
			make(map[int64][]float64, 0),
			make(map[int64][]float64, 0),
		},
	}
	atomic.StoreInt32(&metricLogger.ActiveID, 0)
	return metricLogger
}

// Setup sets up a periodic process to calculate and send metrics
func (g *MetricLoggerGroup) Setup() {
	logging.Info("Setup Metric logger Group for %v Refresh period: %v\n", g.Name, g.RefreshPeriod)
	for _, metricLogger := range g.MetricLoggers {
		metricLogger.RefreshMetricLogger()
	}
	ticker := time.NewTicker(g.RefreshPeriod)
	go func() {
		for {
			select {
			case <-ticker.C:
				// TODO: cleanup unused metric loggers
				for _, metricLogger := range g.MetricLoggers {
					metricLogger.RefreshMetricLogger()
				}
			}
		}
	}()
}

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

// TODO: add other info like http method

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
	logging.Info(">>>>>> Process and Clean Metriclogger Values for %v\n", m.Name)
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
	return nil
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

// CalculateMetricStats does what its name says
func CalculateMetricStats(valuesRaw []float64) (*models.MetricStats, error) {
	// Calculate metrics and send it to vamp api

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
	} else {
		logging.Error("Calculation Error: %v\n", calculationErr)
	}

	// P99
	if calculation, calculationErr := stats.Percentile(values, 0.99); calculationErr == nil {
		metricStats.P99 = calculation
	} else {
		logging.Error("Calculation Error: %v\n", calculationErr)
	}

	// P95
	if calculation, calculationErr := stats.Percentile(values, 0.95); calculationErr == nil {
		metricStats.P95 = calculation
	} else {
		logging.Error("Calculation Error: %v\n", calculationErr)
	}

	// P75
	if calculation, calculationErr := stats.Percentile(values, 0.75); calculationErr == nil {
		metricStats.P75 = calculation
	} else {
		logging.Error("Calculation Error: %v\n", calculationErr)
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
