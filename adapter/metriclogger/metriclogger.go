package metriclogger

import (
	"fmt"
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
	"latency":      NewMetricLoggerGroup("latency", "value"),
	"response-200": NewMetricLoggerGroup("response-200", "categorical"),
	"response-500": NewMetricLoggerGroup("response-500", "categorical"),
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
	NumberOfElements  float64
	Average           float64
	StandardDeviation float64
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
	metricLogger.Setup() // there should be a way to delete when it is no longer needed.
	return metricLogger
}

func (m *MetricLogger) Setup() {
	logging.Info("Setup Metric logger for %v Refresh period: %v\n", m.Name, m.RefreshPeriod)
	m.RefreshMetricLogger()
	ticker := time.NewTicker(m.RefreshPeriod)
	go func() {
		for {
			select {
			case <-ticker.C:
				m.RefreshMetricLogger()
			}
		}
	}()
}

// Push add data to the active bucket
func (m *MetricLogger) Push(timestamp int64, value float64) {
	id := atomic.LoadInt32(&m.ActiveID)
	if _, ok := m.ValueMaps[id][timestamp]; !ok {
		m.ValueMaps[id][timestamp] = []float64{value}
		return
	}
	switch m.MetricType {
	case Categorical:
		m.ValueMaps[id][timestamp][0] += value
	case Valued:
		m.ValueMaps[id][timestamp] = append(m.ValueMaps[id][timestamp], value)
	default: // default is valued
		m.ValueMaps[id][timestamp] = append(m.ValueMaps[id][timestamp], value)
	}

}

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

// RefreshMetricLogger
func (m *MetricLogger) RefreshMetricLogger() error {
	logging.Info("Process and Clean Metriclogger Values for %v\n", m.Name)
	metricValues := m.MergeValuesOfNonActiveBucketsWithTimeBasedFiltering()
	processError := m.ProcessMetricLogger(metricValues)
	if processError != nil {
		return processError
	}
	id := atomic.LoadInt32(&m.ActiveID)
	oldestID := (int(id) - 1) % len(m.ValueMaps)
	m.ValueMaps[oldestID] = make(map[int64][]float64, 0)
	atomic.StoreInt32(&m.ActiveID, int32(oldestID))
	return nil
}

// ProcessMetricLogger processes metrics and trigger send metrics
func (m *MetricLogger) ProcessMetricLogger(metricValues *MetricValues) error {
	allValues := make([]float64, 0, len(metricValues.Values))
	timeLength := metricValues.EndTime - metricValues.StartTime + 1
	timeSeriesAsSum := make([]float64, timeLength, timeLength)
	timeSeriesAsAvg := make([]float64, timeLength, timeLength)
	for t, v := range metricValues.Values {
		allValues = append(allValues, v...)
		sum, _ := stats.Sum(v)
		avg, _ := stats.Mean(v)
		index := t - metricValues.StartTime
		if index >= 0 && index < timeLength {
			timeSeriesAsSum[index] = sum
			timeSeriesAsAvg[index] = avg
		}
	}
	CalculateMetricStatsAndSend(allValues)
	CalculateMetricStatsAndSend(timeSeriesAsSum)
	CalculateMetricStatsAndSend(timeSeriesAsAvg)

	return nil
}

// CalculateMetricStatsAndSend does what its name says
func CalculateMetricStatsAndSend(values []float64) error {
	// Calculate metrics and send it to vamp api
	metricStats := MetricStats{
		NumberOfElements:  float64(len(values)),
		Average:           0,
		StandardDeviation: 0,
	}
	average, meanError := stats.Mean(values)
	if meanError != nil {
		logging.Error("Mean Error: %v\n", meanError)
	}
	metricStats.Average = average

	standardDeviation, standardDeviationError := stats.StandardDeviation(values)
	if standardDeviationError != nil {
		logging.Error("StandardDeviation Error: %v\n", standardDeviationError)
	}
	metricStats.StandardDeviation = standardDeviation
	logging.Info("Metrics should be sent now: %v\n", metricStats)
	// go sendMetric( ... )
	return nil
}
