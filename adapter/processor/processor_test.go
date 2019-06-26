package processor_test

import (
	"testing"

	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/configurator"
	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/models"
	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/processor"
	"github.com/stretchr/testify/assert"
)

func generateCookie(experimentName, userID, subsetName string) string {
	cookieExtra := "guest_token=ImF3SzNsTlMyaHVLLUVGT3lqZXFjRFExNTYwOTQ0ODY2MTI4Ig%3D%3D327ef99d0408d7d2738b87379d80015a760d8594;_vshop_session=cURvN3QwQnFicy9JWWZWNWkzcy9vWDBzd09WN3RHTkZsWUs0eEwvblIwdWxwMEVjODlOVGhGbGlNNFN0US9ZV3hDRkFBTTNic0cwMnZuSVdtNS9ZaVE4Z29neGpkbFIxNVV5OG4xU3pvN3RxR3JQSHNGQ2Z2NFcyQ2lzUFdYc1d5RFpEQTVlemJmenQ4NUhWWE1tV1B2MDdkc0N4RlNpQjZ5eEJ0Y3l5VWlkREIvby9JV1N0cms4a1RzbnV6Z21sLS1Mc1lEdHJTaEk1UDQrNnJSdWhmT0l3PT0%3D--35e006358222b813f496eb7e3ab4136886f1d0d0"
	return experimentName + "_user=" + userID + ";" + experimentName + "=" + subsetName + ";" + cookieExtra
}

func TestProcessIntance1(t *testing.T) {
	configurator.SetupConfigurator()

	experimentName := "ex-1"
	subset1Name := "dest-1-9191-subset1"
	subset2Name := "dest-1-9191-subset2"
	user1ID := "50244035-d34d-4692-98fa-d473181fbbbe"
	user2ID := "3c445470-721f-48d2-ad4a-02de5d19988c"
	user3ID := "3cdeeadd-5c27-4b24-b1c6-fc4e09f8ccc7"
	landingPath := "/"
	targetPath := "/target"
	destination := "demo-app"

	experimentConfigurationMap := make(map[string]models.ExperimentConfiguration)
	experimentConfigurationMap[experimentName] = models.ExperimentConfiguration{
		LandingPath: landingPath,
		TargetPath:  targetPath,
		Subsets: map[string]bool{
			subset1Name: true,
			subset2Name: true,
		},
	}

	configurator.ExperimentConfigurations0.ExperimentConfigurationMap = experimentConfigurationMap

	logInstance1 := &models.LogInstance{
		Destination: destination,
		URL:         targetPath,
		Cookie:      generateCookie(experimentName, user1ID, subset1Name),
	}

	experimentConfigurations := configurator.GetExperimentConfigurations()
	processor.ProcessInstance(experimentConfigurations, logInstance1)

	assert.Equal(t, 1, processor.ExperimentLogs[experimentName].SubsetLogs[subset1Name].UserLogs[user1ID])

	logInstance2 := &models.LogInstance{
		Destination: destination,
		URL:         targetPath,
		Cookie:      generateCookie(experimentName, user2ID, subset1Name),
	}
	processor.ProcessInstance(experimentConfigurations, logInstance2)

	assert.Equal(t, 1, processor.ExperimentLogs[experimentName].SubsetLogs[subset1Name].UserLogs[user1ID])

	logInstance3 := &models.LogInstance{
		Destination: destination,
		URL:         targetPath,
		Cookie:      generateCookie(experimentName, user3ID, subset2Name),
	}

	processor.ProcessInstance(experimentConfigurations, logInstance3)
	assert.Equal(t, 1, processor.ExperimentLogs[experimentName].SubsetLogs[subset2Name].UserLogs[user3ID])

	logInstance4 := &models.LogInstance{
		Destination: destination,
		URL:         targetPath,
		Cookie:      generateCookie(experimentName, user1ID, subset1Name),
	}

	processor.ProcessInstance(experimentConfigurations, logInstance4)
	assert.Equal(t, 2, processor.ExperimentLogs[experimentName].SubsetLogs[subset1Name].UserLogs[user1ID])

}
