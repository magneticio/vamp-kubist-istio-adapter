package metriclogger

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/magneticio/vampkubistcli/logging"
	"github.com/montanaflynn/stats"
)

const DefaultRefreshPeriod = 30 * time.Second

type MetricLoggerGroup struct {
	Name          string
	RefreshPeriod time.Duration
	MetricLoggers map[string]*MetricLogger
}

type MetricLogger struct {
	Name          string
	Destination   string
	Port          string
	Subset        string
	Values0       MetricValues
	Values1       MetricValues
	ActiveID      int32
	RefreshPeriod time.Duration
}

type MetricValues struct {
	Values []float64
}

type MetricStats struct {
	NumberOfElements  float64
	Average           float64
	StandardDeviation float64
}

func NewMetricLoggerGroup(metricName string) *MetricLoggerGroup {
	metricLoggerGroup := &MetricLoggerGroup{
		Name:          metricName,
		MetricLoggers: make(map[string]*MetricLogger),
		RefreshPeriod: DefaultRefreshPeriod,
	}
	// metricLoggerGroup.Setup()
	return metricLoggerGroup
}

func (g *MetricLoggerGroup) GetMetricLogger(destination string, port string, subset string) *MetricLogger {
	key := fmt.Sprintf("%v:%v/%v", destination, port, subset)
	if _, ok := g.MetricLoggers[key]; !ok {
		g.MetricLoggers[key] = NewMetricLogger(destination, port, subset, g.Name, DefaultRefreshPeriod)
	}
	return g.MetricLoggers[key]
}

func NewMetricLogger(destination string, port string, subset string, metricName string, refreshPeriod time.Duration) *MetricLogger {
	metricLogger := &MetricLogger{
		Name:          metricName,
		Destination:   destination,
		Port:          port,
		Subset:        subset,
		RefreshPeriod: refreshPeriod,
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

func (m *MetricLogger) Push(timestamp int64, value float64) {
	if atomic.LoadInt32(&m.ActiveID) == 0 {
		m.Values0.Values = append(m.Values0.Values, value)
	} else {
		m.Values1.Values = append(m.Values1.Values, value)
	}
}

func (m *MetricLogger) RefreshMetricLogger() error {
	logging.Info("Process and Clean Metriclogger Values for %v\n", m.Name)
	if atomic.LoadInt32(&m.ActiveID) == 0 {
		atomic.StoreInt32(&m.ActiveID, 1)
		processError := m.ProcessMetricLogger(&m.Values0)
		if processError != nil {
			return processError
		}
		m.Values0 = MetricValues{}
		return nil
	} else {
		atomic.StoreInt32(&m.ActiveID, 0)
		processError := m.ProcessMetricLogger(&m.Values1)
		if processError != nil {
			return processError
		}
		m.Values1 = MetricValues{}
		return nil
	}
}

// ProcessMetricLogger processes metrics and trigger send metrics
func (m *MetricLogger) ProcessMetricLogger(metricValues *MetricValues) error {
	// Calculate metrics and send it to vamp api
	metricStats := MetricStats{
		NumberOfElements:  float64(len(metricValues.Values)),
		Average:           0,
		StandardDeviation: 0,
	}
	average, meanError := stats.Mean(metricValues.Values)
	if meanError != nil {
		logging.Error("Mean Error: %v\n", meanError)
	}
	metricStats.Average = average

	standardDeviation, standardDeviationError := stats.StandardDeviation(metricValues.Values)
	if standardDeviationError != nil {
		logging.Error("StandardDeviation Error: %v\n", standardDeviationError)
	}
	metricStats.StandardDeviation = standardDeviation
	logging.Info("Metrics should be sent now: %v\n", metricStats)
	// go sendMetric( ... )
	return nil
}
