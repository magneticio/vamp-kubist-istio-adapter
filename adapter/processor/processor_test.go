package processor_test

import (
	"testing"
	"time"

	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/configurator"
	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/models"
	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/processor"
	"github.com/stretchr/testify/assert"
)

func generateCookie(experimentName, userID, subsetName string) string {
	cookieExtra := "guest_token=ImF3SzNsTlMyaHVLLUVGT3lqZXFjRFExNTYwOTQ0ODY2MTI4Ig%3D%3D327ef99d0408d7d2738b87379d80015a760d8594;_vshop_session=cURvN3QwQnFicy9JWWZWNWkzcy9vWDBzd09WN3RHTkZsWUs0eEwvblIwdWxwMEVjODlOVGhGbGlNNFN0US9ZV3hDRkFBTTNic0cwMnZuSVdtNS9ZaVE4Z29neGpkbFIxNVV5OG4xU3pvN3RxR3JQSHNGQ2Z2NFcyQ2lzUFdYc1d5RFpEQTVlemJmenQ4NUhWWE1tV1B2MDdkc0N4RlNpQjZ5eEJ0Y3l5VWlkREIvby9JV1N0cms4a1RzbnV6Z21sLS1Mc1lEdHJTaEk1UDQrNnJSdWhmT0l3PT0%3D--35e006358222b813f496eb7e3ab4136886f1d0d0"
	return experimentName + "_user=" + userID + ";" + experimentName + "=" + subsetName + ";" + cookieExtra
}

func TestProcessRegex(t *testing.T) {
	targetPath := "/cart?variant_id=1"
	regex, err := processor.GetRegexForStartsWithPath(targetPath)
	assert.Equal(t, nil, err)
	check1 := regex.MatchString(targetPath)
	assert.Equal(t, true, check1)
}

func TestProcessIntance1(t *testing.T) {
	experimentName := "ex-1"
	subset1Name := "dest-1-9191-subset1"
	subset2Name := "dest-1-9191-subset2"
	user1ID := "50244035-d34d-4692-98fa-d473181fbbbe"
	user2ID := "3c445470-721f-48d2-ad4a-02de5d19988c"
	user3ID := "3cdeeadd-5c27-4b24-b1c6-fc4e09f8ccc7"
	targetPath := "/target"
	destination := "demo-app"

	experimentConfigurationMap := make(map[string]models.ExperimentConfiguration)
	experimentConfigurationMap[experimentName] = models.ExperimentConfiguration{
		Subsets: map[string]string{
			subset1Name: targetPath,
			subset2Name: targetPath,
		},
	}

	configurator.ExperimentConfigurations0.ExperimentConfigurationMap = experimentConfigurationMap

	processor.SendExperimentLoggers = false
	// This also runs setup configurator
	go processor.RunProcessor()

	logInstance1 := &models.LogInstance{
		Timestamp:         time.Now().Unix(),
		Destination:       destination,
		DestinationLabels: make(map[string]string, 0),
		Values:            make(map[string]interface{}, 1),
	}
	logInstance1.Values["url"] = targetPath
	logInstance1.Values["cookies"] = generateCookie(experimentName, user1ID, subset1Name)

	experimentConfigurations := configurator.GetExperimentConfigurations()
	processor.ProcessInstanceForExperiments(experimentConfigurations, logInstance1)

	assert.Equal(t, 1, processor.GetMergedExperimentLoggers().ExperimentLogs[experimentName].SubsetLogs[subset1Name].UserLogs[user1ID])

	logInstance2 := &models.LogInstance{
		Timestamp:         time.Now().Unix(),
		Destination:       destination,
		DestinationLabels: make(map[string]string, 0),
		Values:            make(map[string]interface{}, 1),
	}
	logInstance2.Values["url"] = targetPath
	logInstance2.Values["cookies"] = generateCookie(experimentName, user2ID, subset1Name)

	processor.ProcessInstanceForExperiments(experimentConfigurations, logInstance2)

	assert.Equal(t, 1, processor.GetMergedExperimentLoggers().ExperimentLogs[experimentName].SubsetLogs[subset1Name].UserLogs[user1ID])

	logInstance3 := &models.LogInstance{
		Timestamp:         time.Now().Unix(),
		Destination:       destination,
		DestinationLabels: make(map[string]string, 0),
		Values:            make(map[string]interface{}, 1),
	}
	logInstance3.Values["url"] = targetPath
	logInstance3.Values["cookies"] = generateCookie(experimentName, user3ID, subset2Name)

	processor.ProcessInstanceForExperiments(experimentConfigurations, logInstance3)
	assert.Equal(t, 1, processor.GetMergedExperimentLoggers().ExperimentLogs[experimentName].SubsetLogs[subset2Name].UserLogs[user3ID])

	logInstance4 := &models.LogInstance{
		Timestamp:         time.Now().Unix(),
		Destination:       destination,
		DestinationLabels: make(map[string]string, 0),
		Values:            make(map[string]interface{}, 1),
	}
	logInstance4.Values["url"] = targetPath
	logInstance4.Values["cookies"] = generateCookie(experimentName, user1ID, subset1Name)

	processor.ProcessInstanceForExperiments(experimentConfigurations, logInstance4)
	assert.Equal(t, 2, processor.GetMergedExperimentLoggers().ExperimentLogs[experimentName].SubsetLogs[subset1Name].UserLogs[user1ID])

	time.Sleep(70 * time.Second)
}
