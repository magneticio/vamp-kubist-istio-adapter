package subsetmapper

import (
	"errors"
	"fmt"
	"io/ioutil"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mhausenblas/kubecuddler"
	combinations "github.com/mxschmitt/golang-combinations"
	"gopkg.in/yaml.v2"

	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/models"
	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/vampclientprovider"
	client "github.com/magneticio/vampkubistcli/client"
	"github.com/magneticio/vampkubistcli/logging"
	clientmodels "github.com/magneticio/vampkubistcli/models"
)

const RefreshPeriod = 30 * time.Second

type SubsetMapper struct {
	DestinationsSubsetMap0 *clientmodels.DestinationsSubsetsMap
	DestinationsSubsetMap1 *clientmodels.DestinationsSubsetsMap
	activeID               int32
	vpc                    vampclientprovider.IVampClientProvider
	restClient             client.IRestClient
}

func New() *SubsetMapper {
	return &SubsetMapper{
		DestinationsSubsetMap0: nil,
		DestinationsSubsetMap1: nil,
		activeID:               0,
		vpc:                    vampclientprovider.New(),
		restClient:             nil,
	}
}

type SubsetInfo struct {
	DestinationName string
	SubsetWithPorts clientmodels.SubsetToPorts
}

// GetRestClient gets or creates rest client
func (sm *SubsetMapper) GetRestClient() (client.IRestClient, error) {
	if sm.restClient != nil {
		return sm.restClient, nil
	}
	restClient, restCLientError := sm.vpc.GetRestClient()
	if restCLientError != nil {
		return nil, errors.New("Rest Client can not be initiliazed")
	}
	sm.restClient = restClient

	return restClient, nil
}

// GetDesitinationsSubsetsMap returns active subset map
func (sm *SubsetMapper) GetDestinationsSubsetsMap() *clientmodels.DestinationsSubsetsMap {
	if atomic.LoadInt32(&sm.activeID) == 0 {
		return sm.DestinationsSubsetMap0
	}
	return sm.DestinationsSubsetMap1
}

// RefreshDestinationsSubsetsMap updates the subset map
func (sm *SubsetMapper) RefreshDestinationsSubsetsMap() error {
	logging.Info("Refresh RefreshDestinationsSubsetsMap at: %v\n", time.Now())
	restClient, restCLientError := sm.GetRestClient()
	if restCLientError != nil {
		return errors.New("Rest Client can not be initiliazed")
	}

	values := sm.vpc.GetConfigValues()
	namespace := values["virtual_cluster"]
	if atomic.LoadInt32(&sm.activeID) == 0 {
		destinationsSubsetsMap, err := restClient.GetSubsetMap(values)
		logging.Info("destinationsSubsetsMap %v\n", destinationsSubsetsMap)
		if err != nil {
			return err
		}
		sm.DestinationsSubsetMap1 = destinationsSubsetsMap
		atomic.StoreInt32(&sm.activeID, 1)
		go ApplyLabelsToInstance(sm.DestinationsSubsetMap1.Labels, namespace)
		return nil
	} else {
		destinationsSubsetsMap, err := restClient.GetSubsetMap(values)
		logging.Info("destinationsSubsetsMap %v\n", destinationsSubsetsMap)
		if err != nil {
			return err
		}
		sm.DestinationsSubsetMap0 = destinationsSubsetsMap
		atomic.StoreInt32(&sm.activeID, 0)
		go ApplyLabelsToInstance(sm.DestinationsSubsetMap1.Labels, namespace)
		return nil
	}
}

// Setup sets up period updates
func (sm *SubsetMapper) Setup() {
	logging.Info("Subset Mapper Setup at %v Refresh period: %v\n", time.Now(), RefreshPeriod)
	sm.RefreshDestinationsSubsetsMap()
	ticker := time.NewTicker(RefreshPeriod)
	go func() {
		for {
			select {
			case <-ticker.C:
				sm.RefreshDestinationsSubsetsMap()
			}
		}
	}()
}

// GetSubsetByLabels return all possible subsets for given labels
func (sm *SubsetMapper) GetSubsetByLabels(destination string, labels map[string]string) []SubsetInfo {
	destinationsSubsetsMap := sm.GetDestinationsSubsetsMap()
	if destinationsSubsetsMap == nil {
		logging.Error("DestinationsSubsetsMap is still nil")
		return []SubsetInfo{}
	}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	labelCombinations := combinations.All(keys)

	subsetList := make([]SubsetInfo, 0)

	for _, combination := range labelCombinations {

		sort.Strings(combination) //sort by key
		var sb strings.Builder
		for i, key := range combination {
			sb.WriteString(key)
			sb.WriteString(":")
			sb.WriteString(labels[key])
			if i < len(combination)-1 {
				sb.WriteString("\n")
			}
		}
		labelsMapString := sb.String()
		// logging.Info("labelsMapString: __%v__\n", labelsMapString)
		if destination != "" {
			destinationData, existDestination := destinationsSubsetsMap.DestinationsMap[destination]
			if !existDestination {
				continue
			}
			subsetData, existSubset := destinationData.Map[labelsMapString]
			if !existSubset {
				continue
			}
			destinationName := destinationData.DestinationName
			subsetName := subsetData.Subset
			subsetPorts := subsetData.Ports
			// logging.Info("destinationData.Map: __%v__\n", destinationData.Map)
			// logging.Info("Subset Ports d( %v ) s( %v ) p( %v )\n", destination, subsetName, subsetPorts)
			subsetWithPorts := clientmodels.SubsetToPorts{
				Subset: subsetName,
				Ports:  subsetPorts,
			}
			subsetList = append(subsetList, SubsetInfo{
				DestinationName: destinationName,
				SubsetWithPorts: subsetWithPorts,
			})
		} else {
			for _, destinationMap := range destinationsSubsetsMap.DestinationsMap {
				subsetData, existSubset := destinationMap.Map[labelsMapString]
				if !existSubset {
					continue
				}
				destinationName := destinationMap.DestinationName
				subsetName := subsetData.Subset
				subsetPorts := subsetData.Ports
				// logging.Info("D Subset Ports %v\n", subsetPorts)
				subsetWithPorts := clientmodels.SubsetToPorts{
					Subset: subsetName,
					Ports:  subsetPorts,
				}
				subsetList = append(subsetList, SubsetInfo{
					DestinationName: destinationName,
					SubsetWithPorts: subsetWithPorts,
				})
			}
		}
	}
	return subsetList
}

/*
# example instance for template logentry
apiVersion: "config.istio.io/v1alpha2"
kind: instance
metadata:
  name: vamplog
  namespace: istio-system
spec:
  template: logentry
  params:
    severity: '"info"'
    timestamp: request.time
    variables:
      source: source.labels["app"] | source.workload.name | "unknown"
      user: source.user | "unknown"
      destination: destination.labels["app"] | destination.name | destination.service.name | "unknown"
      destinationPort: destination.port | 0
      responseCode: response.code | 0
      responseSize: response.size | 0
      latency: response.duration | "0ms"
      url: request.path | ""
      cookies: request.headers["cookie"] | ""
	  destinationVersion: destination.labels["version"] | "unknown"

*/

// GenerateInstanceWithLogEntryTemplate generate instance template with given labels and namespace
func GenerateInstanceWithLogEntryTemplate(labels []string, namespace string) *models.Instance {
	variables := map[string]string{
		"source":               "source.labels[\"app\"] | source.workload.name | \"\"",
		"user":                 "source.user | \"\"",
		"destinationName":      "destination.service.name | \"\"",
		"destinationNamespace": "destination.namespace | \"\"",
		"destinationPort":      "destination.port | 0",
		"responseCode":         "response.code | 0",
		"apiProtocol":          "api.protocol | \"\"", // http, https, or grpc
		"requestMethod":        "request.method | \"\"",
		"responseSize":         "response.size | 0",
		"responseDuration":     "response.duration | \"0ms\"",
		"url":                  "request.path | \"\"",
		"cookies":              "request.headers[\"cookie\"] | \"\"",
	}

	labelPrefix := "label"
	for _, label := range labels {
		key := fmt.Sprintf("%s_%s", labelPrefix, label)
		variables[key] = fmt.Sprintf("destination.labels[\"%s\"] | \"\"", label)
	}

	params := models.InstanceParams{
		Severity:  "\"info\"",
		Timestamp: "request.time",
		Variables: variables,
	}

	spec := models.InstanceSpec{
		Template: "logentry",
		Params:   params,
	}

	instance := &models.Instance{
		APIVersion: "config.istio.io/v1alpha2",
		Kind:       "instance",
		Metadata: map[string]string{
			"name":      fmt.Sprintf("%s-%s", "vamplog", namespace),
			"namespace": "istio-system",
		},
		Spec: spec,
	}

	return instance
}

// ApplyLabelsToInstance generate and apply instance crd to
// local configured kubectl
// This is a temporary solution until istio fixes destination labels
// This pull request fixes it: https://github.com/istio/api/pull/925
// but it isn't arrived at main release, it should be tested with new release
func ApplyLabelsToInstance(labels []string, namespace string) error {
	instance := GenerateInstanceWithLogEntryTemplate(labels, namespace)
	instanceBytes, marshalError := yaml.Marshal(instance)
	if marshalError != nil {
		return marshalError
	}
	writeError := ioutil.WriteFile("/tmp/instance.yaml", instanceBytes, 0644)
	if writeError != nil {
		return writeError
	}

	res, kubeErr := kubecuddler.Kubectl(true, true, "kubectl", "apply", "-f", "/tmp/instance.yaml")

	if kubeErr != nil {
		return kubeErr
	}

	logging.Info(res)

	return nil
}
