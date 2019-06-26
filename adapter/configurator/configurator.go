package configurator

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/models"
)

var ExperimentConfigurations0 models.ExperimentConfigurations
var ExperimentConfigurations1 models.ExperimentConfigurations

var activeConfigurationID int32

const RefreshPeriod = 30 * time.Second

func GetExperimentConfigurations() *models.ExperimentConfigurations {
	if atomic.LoadInt32(&activeConfigurationID) == 0 {
		return &ExperimentConfigurations0
	}
	return &ExperimentConfigurations1
}

// GenerateNewExperimentConfigurations gets experiments from the service
func GenerateNewExperimentConfigurations() models.ExperimentConfigurations {
	// for a virtual cluster get list of experiments
	// loop through experiment configurations
	// merge results into a map
	return ExperimentConfigurations0
}

func RefreshExperimentConfigurations() error {
	fmt.Println("Refresh Experiment Configurations at: ", time.Now())
	if atomic.LoadInt32(&activeConfigurationID) == 0 {
		ExperimentConfigurations1 = GenerateNewExperimentConfigurations()
		atomic.StoreInt32(&activeConfigurationID, 1)
		return nil
	} else {
		ExperimentConfigurations0 = GenerateNewExperimentConfigurations()
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
