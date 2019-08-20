package healthmetrics

import (
	"errors"
	"time"

	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/models"
	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/vampclientprovider"
	kubeclient "github.com/magneticio/vampkubistcli/kubernetes"
	"github.com/magneticio/vampkubistcli/logging"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
)

// MetricsReadPeriod is interval between metrics retrieval
const MetricsReadPeriod = 10 * time.Second

var k8sclient *k8s.Clientset

// Setup setups periodic read of health metrics
func Setup(ch chan *models.LogInstance) {
	logging.Info("Setup reading k8s health metrics at %v with period %v\n", time.Now(), MetricsReadPeriod)

	var err error
	k8sclient, err = kubeclient.K8sClient.Get("")
	if err != nil {
		logging.Error("Cannot get k8s client: %v", err)
		return
	}

	kubeclient.IsKubeClientInCluster = true
	ProcessHealthMetrics(ch)
	ticker := time.NewTicker(MetricsReadPeriod)
	go func() {
		for {
			select {
			case <-ticker.C:
				ProcessHealthMetrics(ch)
			}
		}
	}()
}

// ProcessHealthMetrics reads health data from K8s and send them to adapter's processor
func ProcessHealthMetrics(ch chan *models.LogInstance) error {
	ns := vampclientprovider.VirtualCluster

	deps, err := k8sclient.AppsV1().Deployments(ns).List(metav1.ListOptions{})
	if err != nil {
		logging.Error("Cannot get list of deployments for namespace %v - %v", ns, err)
		return err
	}

	for i := 0; i < len(deps.Items); i++ {
		logInstance := &models.LogInstance{
			Timestamp:         time.Now().Unix(),
			DestinationLabels: deps.Items[i].Labels,
			Values: map[string]interface{}{
				"ObservedGeneration":  deps.Items[i].Status.ObservedGeneration,
				"Replicas":            deps.Items[i].Status.Replicas,
				"UpdatedReplicas":     deps.Items[i].Status.UpdatedReplicas,
				"ReadyReplicas":       deps.Items[i].Status.ReadyReplicas,
				"AvailableReplicas":   deps.Items[i].Status.AvailableReplicas,
				"UnavailableReplicas": deps.Items[i].Status.UnavailableReplicas,
			},
		}
		av, err := getAvailability(deps.Items[i].Status.Conditions)
		if err == nil {
			logging.Info("Cannot get availability for namespace %v - %v", ns, err)
		} else {
			logInstance.Values["Availability"] = av
		}
		ch <- logInstance
	}

	return nil
}

func getAvailability(cs []appsv1.DeploymentCondition) (int32, error) {
	for _, c := range cs {
		if c.Type == appsv1.DeploymentAvailable {
			switch c.Status {
			case corev1.ConditionTrue:
				return 1, nil
			case corev1.ConditionFalse:
				return 0, nil
			default:
				return 0, errors.New("Status unknown")
			}
		}
	}
	return 0, errors.New("There is no condition with deployment available type")
}
