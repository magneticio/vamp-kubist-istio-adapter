package subsetmapper

import (
	"errors"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/vampclientprovider"
	"github.com/magneticio/vampkubistcli/logging"
	clientmodels "github.com/magneticio/vampkubistcli/models"
	combinations "github.com/mxschmitt/golang-combinations"
)

var DestinationsSubsetMap0 clientmodels.DestinationsSubsetsMap
var DestinationsSubsetMap1 clientmodels.DestinationsSubsetsMap

var activeID int32

const RefreshPeriod = 30 * time.Second

// GetDesitinationsSubsetsMap returns active subset map
func GetDestinationsSubsetsMap() *clientmodels.DestinationsSubsetsMap {
	if atomic.LoadInt32(&activeID) == 0 {
		return &DestinationsSubsetMap0
	}
	return &DestinationsSubsetMap1
}

// RefreshDestinationsSubsetsMap updates the subset map
func RefreshDestinationsSubsetsMap() error {
	logging.Info("Refresh Experiment Configurations at: %v\n", time.Now())
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
		if err != nil {
			return err
		}
		DestinationsSubsetMap1 = *destinationsSubsetsMap
		atomic.StoreInt32(&activeID, 1)
		return nil
	} else {
		destinationsSubsetsMap, err := restClient.GetSubsetMap(values)
		if err != nil {
			return err
		}
		DestinationsSubsetMap0 = *destinationsSubsetsMap
		atomic.StoreInt32(&activeID, 0)
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

// GetSubsetByLabels return all possible subsets for given labels
func GetSubsetByLabels(destination string, labels map[string]string) []string {
	destinationsSubsetsMap := GetDestinationsSubsetsMap()
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	labelCombinations := combinations.All(keys)

	subsetList := make([]string, 0)

	for _, combination := range labelCombinations {

		sort.Strings(combination) //sort by key
		var sb strings.Builder
		for i, key := range combination {
			sb.WriteString(key)
			sb.WriteString(":")
			sb.WriteString(labels[key])
			if i != len(labels) {
				sb.WriteString("\n")
			}
		}
		labelsMapString := sb.String()
		subset := destinationsSubsetsMap.DestinationsMap[destination].Map[labelsMapString].Subset
		subsetList = append(subsetList, subset)
	}
	return subsetList
}
