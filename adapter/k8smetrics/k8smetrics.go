package k8smetrics

import (
	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/models"
	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/processor"
	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/vampclientprovider"
	kubernetes "github.com/magneticio/vampkubistcli/kubernetes"
	"github.com/magneticio/vampkubistcli/logging"
	"time"
)

// MetricsReadPeriod is interval between metrics retrieval
const MetricsReadPeriod = 30 * time.Second

// ProcessK8sMetrics reads metrics from K8s metric server and send them to adapter's processor
func ProcessK8sMetrics() error {
	metrics, err := kubernetes.GetSimpleMetrics("", vampclientprovider.VirtualCluster)
	if err != nil {
		return err
	}
	for i := 0; i < len(metrics); i++ {
		logInstance := &models.LogInstance{
			Timestamp:         time.Now().Unix(),
			DestinationLabels: metrics[i].Labels,
			Values:            map[string]interface{}{"CPU": metrics[i].CPU, "Memory": metrics[i].Memory},
		}
		processor.LogInstanceChannel <- logInstance
	}
	return nil
}

func SetupK8sMetrics() {
	logging.Info("SetupConfigurator at %v Refresh period: %v\n", time.Now(), MetricsReadPeriod)
	ProcessK8sMetrics()
	ticker := time.NewTicker(MetricsReadPeriod)
	go func() {
		for {
			select {
			case <-ticker.C:
				ProcessK8sMetrics()
			}
		}
	}()
}
