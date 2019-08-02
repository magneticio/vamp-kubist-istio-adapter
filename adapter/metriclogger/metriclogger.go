package metriclogger

import (
	"sync/atomic"
	"time"

	"github.com/magneticio/vampkubistcli/logging"
)

const DefaultRefreshPeriod = 30 * time.Second

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

func NewMetricLogger(destination string, port string, subset string, metricName string, refreshPeriod time.Duration) *Metriclogger {
	metricLogger := &Metriclogger{
		Name:          metricName,
		Destination:   destination,
		Port:          port,
		Subset:        subset,
		refreshPeriod: refreshPeriod,
	}
	atomic.StoreInt32(&metricLogger.ActiveID, 0)
	return metricLogger
}

func (m *Metriclogger) Setup() {
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

func (m *Metriclogger) RefreshMetricLogger() error {
	logging.Info("Process and Clean Metriclogger Values for \n", m.Name)
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

func (m *Metriclogger) ProcessMetricLogger(metricValues *MetricValues) error {
	// Calculate metrics and send it to vamp api
	return nil
}
