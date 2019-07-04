package configurator

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/models"
	"github.com/magneticio/vampkubistcli/client"
	"github.com/magneticio/vampkubistcli/logging"
	"github.com/spf13/viper"
)

var ExperimentConfigurations0 models.ExperimentConfigurations
var ExperimentConfigurations1 models.ExperimentConfigurations

var activeConfigurationID int32

const RefreshPeriod = 30 * time.Second

var _restClient *client.RestClient
var URL string
var Token string
var APIVersion string
var Cert string
var TokenStore client.TokenStore
var Project string
var Cluster string
var VirtualCluster string

func initViperConfig(path string) {
	viper.SetConfigName("vamp-config") // name of config file (without extension)
	viper.AddConfigPath(path)          // path to look for the config file in
	viper.AddConfigPath(".")           // optionally look for config in the working directory
	err := viper.ReadInConfig()        // Find and read the config file
	if err != nil {                    // Handle errors reading the config file
		// panic(fmt.Errorf("Fatal error config file: %s \n", err))
		logging.Error("Fatal error config file: %s \n", err)
	}
}

func getRestClient() (*client.RestClient, error) {
	if _restClient != nil {
		return _restClient, nil
	}
	URL = viper.GetString("url")
	Token = viper.GetString("token")
	APIVersion = viper.GetString("apiversion")
	/* certEncoded := viper.GetString("cert")
	certByte, certDecodeError := base64.StdEncoding.DecodeString(certEncoded)
	if certDecodeError != nil {
		return nil, certDecodeError
	}
	Cert = string(certByte) */
	Cert = viper.GetString("cert")
	TokenStore = &client.InMemoryTokenStore{}
	_restClient := client.NewRestClient(URL, Token, APIVersion, false, Cert, &TokenStore)
	if _restClient == nil {
		return nil, errors.New("Rest Client can not be initiliazed")
	}
	return _restClient, nil
}

func GetExperimentConfigurations() *models.ExperimentConfigurations {
	if atomic.LoadInt32(&activeConfigurationID) == 0 {
		return &ExperimentConfigurations0
	}
	return &ExperimentConfigurations1
}

func ParseExperimentConfiguration(sourceAsJson string) (*models.ExperimentConfigurations, error) {
	var experiments []models.Experiment
	err := json.Unmarshal([]byte(sourceAsJson), &experiments)
	if err != nil {
		return nil, err
	}
	experimentConfigurationMap := make(map[string]models.ExperimentConfiguration)
	for _, experiment := range experiments {
		subsets := make(map[string]string)
		for _, destination := range experiment.Specification.Destinations {
			portString := strconv.FormatInt(destination.Port, 10)
			key := destination.Destination + "-" + portString + "-" + destination.Subset
			subsets[key] = destination.Target
		}
		experimentConfigurationMap[experiment.Name] = models.ExperimentConfiguration{
			Subsets: subsets,
		}
	}
	return &models.ExperimentConfigurations{
		ExperimentConfigurationMap: experimentConfigurationMap,
	}, nil
}

// GenerateNewExperimentConfigurations gets experiments from the service
func GenerateNewExperimentConfigurations() (*models.ExperimentConfigurations, error) {
	// "/tmp/documentation1/vamp-config.yaml"
	initViperConfig("/tmp/documentation1")
	restClient, restCLientError := getRestClient()
	if restCLientError != nil {
		return nil, errors.New("Rest Client can not be initiliazed")
	}
	Project = viper.GetString("project")
	Cluster = viper.GetString("cluster")
	VirtualCluster = viper.GetString("virtualcluster")
	values := make(map[string]string)
	values["project"] = Project
	values["cluster"] = Cluster
	values["virtual_cluster"] = VirtualCluster
	listOfExperiments, err := restClient.List("experiment", "json", values, false)
	if err != nil {
		return nil, err
	}
	experimentConfs, confReadError := ParseExperimentConfiguration(listOfExperiments)
	if confReadError != nil {
		return nil, confReadError
	}
	logging.Info("experimentConfs: %v\n", *experimentConfs)
	return experimentConfs, nil
}

func RefreshExperimentConfigurations() error {
	fmt.Println("Refresh Experiment Configurations at: ", time.Now())
	if atomic.LoadInt32(&activeConfigurationID) == 0 {
		experimentConfigurations, confError := GenerateNewExperimentConfigurations()
		if confError != nil {
			return confError
		}
		ExperimentConfigurations1 = *experimentConfigurations
		atomic.StoreInt32(&activeConfigurationID, 1)
		return nil
	} else {
		experimentConfigurations, confError := GenerateNewExperimentConfigurations()
		if confError != nil {
			return confError
		}
		ExperimentConfigurations0 = *experimentConfigurations
		atomic.StoreInt32(&activeConfigurationID, 0)
		return nil
	}
}

func SetupConfigurator() {
	fmt.Println("SetupConfigurator at ", time.Now(), "Refresh period: ", RefreshPeriod)
	RefreshExperimentConfigurations()
	ticker := time.NewTicker(RefreshPeriod)
	go func() {
		for {
			select {
			case <-ticker.C:
				RefreshExperimentConfigurations()
			}
		}
	}()
}
