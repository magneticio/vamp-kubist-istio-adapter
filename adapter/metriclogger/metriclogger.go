package metriclogger

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/magneticio/vampkubistcli/logging"
	"github.com/montanaflynn/stats"
)

const DefaultRefreshPeriod = 30 * time.Second

var MetricDefinitons = map[string]MetricInfo{
	"latency": MetricInfo{
		Type:       Valued,
		NameFormat: "latency",
	},
	"responseCode": MetricInfo{
		Type:       Categorical,
		NameFormat: "response-%v",
	},
}

var MetricLoggerGroupMap = map[string]*MetricLoggerGroup{
	"latency":      NewMetricLoggerGroup("latency", Valued),
	"response-200": NewMetricLoggerGroup("response-200", Categorical),
	"response-500": NewMetricLoggerGroup("response-500", Categorical),
}

type MetricType int

const (
	Valued MetricType = iota
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

type MetricStats struct {
	NumberOfElements  int64
	Average           float64
	StandardDeviation float64
	Sum               float64
	Median            float64
	Min               float64
	Max               float64
	Rate              float64
	P999              float64
	P99               float64
	P95               float64
	P75               float64
}

func NewMetricLoggerGroup(metricName string, metricType MetricType) *MetricLoggerGroup {
	metricLoggerGroup := &MetricLoggerGroup{
		Name:          metricName,
		MetricType:    metricType,
		MetricLoggers: make(map[string]*MetricLogger),
		RefreshPeriod: DefaultRefreshPeriod,
	}
	metricLoggerGroup.Setup()
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
		go metricLogger.RefreshMetricLogger()
	}
	ticker := time.NewTicker(g.RefreshPeriod)
	go func() {
		for {
			select {
			case <-ticker.C:
				// TODO: cleanup unused metric loggers
				for _, metricLogger := range g.MetricLoggers {
					go metricLogger.RefreshMetricLogger()
				}
			}
		}
	}()
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

// TODO: add other info like http method
// GetMetricLoggerName return name to be used as metric identifier
func (m *MetricInfo) GetMetricLoggerNames(name string, value interface{}) []string {
	switch m.Type {
	case Categorical:
		groupName := fmt.Sprintf(m.NameFormat, value)
		return []string{groupName}
	case Valued:
		return []string{name}
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
func (m *MetricLogger) MergeValuesOfNonActiveBucketsWithTimeBasedFiltering() *MetricValues {
	id := atomic.LoadInt32(&m.ActiveID)
	now := time.Now().Unix()
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
	metricValues := m.MergeValuesOfNonActiveBucketsWithTimeBasedFiltering()
	processError := m.ProcessMetricLogger(metricValues)
	if processError != nil {
		return processError
	}
	id := atomic.LoadInt32(&m.ActiveID)
	// TODO: review this logic of calculating oldest id
	oldestID := (int(id) - 1) % len(m.ValueMaps)
	m.ValueMaps[oldestID] = make(map[int64][]float64, 0)
	atomic.StoreInt32(&m.ActiveID, int32(oldestID))
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
	CalculateMetricStatsAndSend(allValues)
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
	CalculateMetricStatsAndSend(timeSeriesAsAvg)

	return nil
}

// CalculateMetricStatsAndSend does what its name says
func CalculateMetricStatsAndSend(valuesRaw []float64) error {
	// Calculate metrics and send it to vamp api

	values := stats.LoadRawData(valuesRaw)
	// NumberOfElements
	metricStats := &MetricStats{
		NumberOfElements: int64(values.Len()),
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

	logging.Info("Metrics should be sent now: %v\n", metricStats)
	// go sendMetric( ... )
	return nil
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
	}
	result = sum / float64(input.Len())
	return result, nil
}