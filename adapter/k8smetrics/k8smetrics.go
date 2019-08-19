package k8smetrics

import (
	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/models"
	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/vampclientprovider"
	kubernetes "github.com/magneticio/vampkubistcli/kubernetes"
	"github.com/magneticio/vampkubistcli/logging"
	"time"
)

// MetricsReadPeriod is interval between metrics retrieval
const MetricsReadPeriod = 30 * time.Second

// ProcessK8sMetrics reads metrics from K8s metric server and send them to adapter's processor
func ProcessK8sMetrics(ch chan *models.LogInstance) error {
	metrics, err := kubernetes.GetSimpleMetrics("", vampclientprovider.VirtualCluster)
	if err != nil {
		logging.Error("Cannot read k8s metrics: %v", err)
		return err
	}
	for i := 0; i < len(metrics); i++ {
		for j := 0; j < len(metrics[i].ContainersMetrics); j++ {
			logInstance := &models.LogInstance{
				Timestamp:         time.Now().Unix(),
				DestinationLabels: metrics[i].Labels,
				Values: map[string]interface{}{
					"CPU":    metrics[i].ContainersMetrics[j].CPU,
					"Memory": metrics[i].ContainersMetrics[j].Memory},
			}
			ch <- logInstance
		}
	}
	return nil
}

// Setup setups periodic read of k8s metrics
func Setup(ch chan *models.LogInstance) {
	logging.Info("SetupConfigurator at %v Refresh period: %v\n", time.Now(), MetricsReadPeriod)
	ProcessK8sMetrics(ch)
	ticker := time.NewTicker(MetricsReadPeriod)
	go func() {
		for {
			select {
			case <-ticker.C:
				ProcessK8sMetrics(ch)
			}
		}
	}()
}
