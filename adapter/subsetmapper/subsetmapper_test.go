package subsetmapper_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/subsetmapper"
	clientmodels "github.com/magneticio/vampkubistcli/models"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

func TestGenerateInstanceWithLogEntryTemplate(t *testing.T) {
	labels := []string{
		"one",
		"two",
	}
	namespace := "test-namespace"
	instance := subsetmapper.GenerateInstanceWithLogEntryTemplate(labels, namespace)
	assert.Equal(t, fmt.Sprintf("%v-%v", "vamplog", namespace), instance.Metadata["name"])
	assert.Equal(t, "istio-system", instance.Metadata["namespace"])
	_, marshalError := yaml.Marshal(instance)
	assert.Equal(t, nil, marshalError)
}

func TestGetSubsetByLabels(t *testing.T) {

	labels := []string{
		"version",
	}
	http := fmt.Sprint("http")

	/*
	  Example:
	  destinationsSubsetsMap &{
	  map[kubist-example-destination:{
	  kubist-example-destination map[
	  version:v0.0.24:{
	  v0-0-24 [{
	  0xc000199540 8080 8080 TCP}]
	  }]
	  }] [version]}
	*/

	destinationMap := map[string]clientmodels.LabelsToPortMap{
		"kubist-example-destination": clientmodels.LabelsToPortMap{
			DestinationName: "kubist-example-destination",
			Map: map[string]clientmodels.SubsetToPorts{
				"version:v0.0.24": clientmodels.SubsetToPorts{
					Subset: "v0-0-24",
					Ports: []clientmodels.DestinationPortSpecification{
						clientmodels.DestinationPortSpecification{
							Name:       &http,
							Port:       8080,
							TargetPort: 8080,
							Protocol:   "TCP",
						},
					},
				},
			},
		},
	}

	sm := &subsetmapper.SubsetMapper{}

	sm.DestinationsSubsetMap0 = &clientmodels.DestinationsSubsetsMap{
		DestinationsMap: destinationMap,
		Labels:          labels,
	}

	sm.DestinationsSubsetMap1 = sm.DestinationsSubsetMap0

	destination0 := "kubist-example-destination"
	destinationLabels0 := map[string]string{
		"version": "v0.0.24",
	}
	subsetInfo0 := sm.GetSubsetByLabels(destination0, destinationLabels0)

	fmt.Printf("Subsets: %v\n", subsetInfo0)

	subsetInfo := subsetInfo0[0]

	fmt.Printf("subsetInfo %v\n", subsetInfo)

	assert.Equal(t, "kubist-example-destination", subsetInfo.DestinationName)
	assert.Equal(t, "v0-0-24", subsetInfo.SubsetWithPorts.Subset)
	assert.Equal(t, int(8080), subsetInfo.SubsetWithPorts.Ports[0].TargetPort)

	port := "8080"
	result := checkSubsetInfoByPort(&subsetInfo, port)

	assert.Equal(t, 0, result)

	port = "8081"
	result = checkSubsetInfoByPort(&subsetInfo, port)

	assert.Equal(t, 1, result)

	port = ""
	result = checkSubsetInfoByPort(&subsetInfo, port)

	assert.Equal(t, 2, result)
}

// This is the logic used inside logInstance processor
func checkSubsetInfoByPort(subsetInfo *subsetmapper.SubsetInfo, port string) int {
	if len(subsetInfo.SubsetWithPorts.Ports) > 0 {
		for _, destWithPorts := range subsetInfo.SubsetWithPorts.Ports {
			if port != "" {
				if strconv.Itoa(destWithPorts.TargetPort) == port {
					return 0
				} else {
					return 1
				}
			} else {
				return 2
			}
		}
	} else {
		return 3
	}
	return -1
}
