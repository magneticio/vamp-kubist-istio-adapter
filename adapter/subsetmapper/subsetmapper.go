package subsetmapper

import (
	"errors"
	"fmt"
	"io/ioutil"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/models"
	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/vampclientprovider"
	"github.com/magneticio/vampkubistcli/logging"
	clientmodels "github.com/magneticio/vampkubistcli/models"
	"github.com/mhausenblas/kubecuddler"
	combinations "github.com/mxschmitt/golang-combinations"
)

var DestinationsSubsetMap0 *clientmodels.DestinationsSubsetsMap
var DestinationsSubsetMap1 *clientmodels.DestinationsSubsetsMap

var activeID int32

const RefreshPeriod = 30 * time.Second

// GetDesitinationsSubsetsMap returns active subset map
func GetDestinationsSubsetsMap() *clientmodels.DestinationsSubsetsMap {
	if atomic.LoadInt32(&activeID) == 0 {
		return DestinationsSubsetMap0
	}
	return DestinationsSubsetMap1
}

// RefreshDestinationsSubsetsMap updates the subset map
func RefreshDestinationsSubsetsMap() error {
	logging.Info("Refresh RefreshDestinationsSubsetsMap at: %v\n", time.Now())
	restClient, restCLientError := vampclientprovider.GetRestClient()
	if restCLientError != nil {
		return errors.New("Rest Client can not be initiliazed")
	}
	values := make(map[string]string)
	values["project"] = vampclientprovider.Project
	values["cluster"] = vampclientprovider.Cluster
	values["virtual_cluster"] = vampclientprovider.VirtualCluster
	if atomic.LoadInt32(&activeID) == 0 {
		destinationsSubsetsMap, err := restClient.GetSubsetMap(values)
		logging.Info("destinationsSubsetsMap %v\n", destinationsSubsetsMap)
		if err != nil {
			return err
		}
		DestinationsSubsetMap1 = destinationsSubsetsMap
		atomic.StoreInt32(&activeID, 1)
		go ApplyLabelsToInstance(DestinationsSubsetMap1.Labels, vampclientprovider.VirtualCluster)
		return nil
	} else {
		destinationsSubsetsMap, err := restClient.GetSubsetMap(values)
		logging.Info("destinationsSubsetsMap %v\n", destinationsSubsetsMap)
		if err != nil {
			return err
		}
		DestinationsSubsetMap0 = destinationsSubsetsMap
		atomic.StoreInt32(&activeID, 0)
		go ApplyLabelsToInstance(DestinationsSubsetMap1.Labels, vampclientprovider.VirtualCluster)
		return nil
	}
}

// Setup sets up period updates
func Setup() {
	logging.Info("Subset Mapper Setup at %v Refresh period: %v\n", time.Now(), RefreshPeriod)
	RefreshDestinationsSubsetsMap()
	ticker := time.NewTicker(RefreshPeriod)
	go func() {
		for {
			select {
			case <-ticker.C:
				RefreshDestinationsSubsetsMap()
			}
		}
	}()
}

type SubsetInfo struct {
	DestinationName string
	SubsetWithPorts clientmodels.SubsetToPorts
}

// GetSubsetByLabels return all possible subsets for given labels
func GetSubsetByLabels(destination string, labels map[string]string) []SubsetInfo {
	destinationsSubsetsMap := GetDestinationsSubsetsMap()
	if destinationsSubsetsMap == nil {
		logging.Error("DestinationsSubsetsMap is still nil")
		fmt.Printf("DestinationsSubsetsMap is still nil")
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
		logging.Info("labelsMapString: __%v__\n", labelsMapString)
		fmt.Printf("labelsMapString: __%v__\n", labelsMapString)
		if destination != "" {
			destinationData, existDestination := destinationsSubsetsMap.DestinationsMap[destination]
			if !existDestination {
				fmt.Printf("destination not exist: %v\n", destination)
				continue
			}
			subsetData, existSubset := destinationData.Map[labelsMapString]
			if !existSubset {
				fmt.Printf("destinationData.Map: %v\n", destinationData.Map)
				fmt.Printf("subset not exist: %v\n", labelsMapString)
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
