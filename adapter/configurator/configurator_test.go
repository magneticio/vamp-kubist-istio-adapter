package configurator_test

import (
	"testing"

	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/configurator"
	"github.com/stretchr/testify/assert"
)

var exterimentListString = `[

    {
        "name": "ex-1",
        "projectName": "project1",
        "specification": {
            "step": 10,
            "vampServiceName": "vs-1",
            "metadata": {},
            "destinations": [
                {
                    "tags": [
                        "test1"
                    ],
                    "port": 9191,
                    "subset": "subset1",
                    "target": "/cart?variant_id=1",
                    "destination": "dest-1"
                },
                {
                    "tags": [
                        "test2"
                    ],
                    "port": 9191,
                    "subset": "subset2",
                    "target": "/cart?variant_id=1",
                    "destination": "dest-1"
                }
            ],
            "period": 1,
            "policies": []
        },
        "virtualClusterName": "kubist-demo",
        "status": {
            "vampServiceStatus": true,
            "currentSpecification": {
                "step": 10,
                "vampServiceName": "vs-1",
                "metadata": {},
                "destinations": [
                    {
                        "tags": [
                            "test1"
                        ],
                        "port": 9191,
                        "subset": "subset1",
                        "target": "/cart?variant_id=1",
                        "destination": "dest-1"
                    },
                    {
                        "tags": [
                            "test2"
                        ],
                        "port": 9191,
                        "subset": "subset2",
                        "target": "/cart?variant_id=1",
                        "destination": "dest-1"
                    }
                ],
                "period": 1,
                "policies": []
            },
            "deploymentsStatuses": {
                "ex-1-dest-1-9191-subset1": true,
                "ex-1-dest-1-9191-subset2": false
            },
            "applicationStatus": false,
            "destinationStatus": true
        },
        "clusterName": "cluster1"
    }
]
`

func TestProcessParseExperimentConfiguration(t *testing.T) {
	experimentConfigurations, err := configurator.ParseExperimentConfiguration(exterimentListString)
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(experimentConfigurations.ExperimentConfigurationMap))
}
