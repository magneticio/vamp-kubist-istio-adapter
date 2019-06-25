package processor_test

import (
	"testing"

	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/models"
	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/processor"
)

func TestProcessIntance1(t *testing.T) {
	logInstance := &models.LogInstance{
		Destination: "service1",
		URL:         "/target",
		Cookie:      "ex-1_user=3c445470-721f-48d2-ad4a-02de5d19988c;ex-1=dest-1-9191-subset2;guest_token=ImF3SzNsTlMyaHVLLUVGT3lqZXFjRFExNTYwOTQ0ODY2MTI4Ig%3D%3D327ef99d0408d7d2738b87379d80015a760d8594;_vshop_session=cURvN3QwQnFicy9JWWZWNWkzcy9vWDBzd09WN3RHTkZsWUs0eEwvblIwdWxwMEVjODlOVGhGbGlNNFN0US9ZV3hDRkFBTTNic0cwMnZuSVdtNS9ZaVE4Z29neGpkbFIxNVV5OG4xU3pvN3RxR3JQSHNGQ2Z2NFcyQ2lzUFdYc1d5RFpEQTVlemJmenQ4NUhWWE1tV1B2MDdkc0N4RlNpQjZ5eEJ0Y3l5VWlkREIvby9JV1N0cms4a1RzbnV6Z21sLS1Mc1lEdHJTaEk1UDQrNnJSdWhmT0l3PT0%3D--35e006358222b813f496eb7e3ab4136886f1d0d0",
	}
	experimentConfigurations := make(map[string]*models.ExperimentConfiguration)
	experimentConfigurations["ex-1"] = &models.ExperimentConfiguration{
		LandingPath: "/",
		TargetPath:  "/target",
		Subsets:     map[string]bool{"dest-1-9191-subset2": true},
	}
	processor.ProcessInstance(experimentConfigurations, logInstance)

	t.Logf("Experiments %v\n", processor.ExperimentLogs["ex-1"].SubsetLogs["dest-1-9191-subset2"])
}
