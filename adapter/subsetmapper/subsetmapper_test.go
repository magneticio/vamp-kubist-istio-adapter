package subsetmapper_test

import (
	"fmt"
	"testing"

	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/subsetmapper"
	"github.com/stretchr/testify/assert"
	"github.com/montanaflynn/stats"
)

func TestGenerateInstanceWithLogEntryTemplate(t *testing.T) {
	valuesRaw = []float64{ 0.1, 0.2 , -0.3}
	values := stats.LoadRawData(valuesRaw)
	instance := subsetmapper.GenerateInstanceWithLogEntryTemplate(labels, namespace)
	assert.Equal(t, fmt.Sprintf("%v-%v", "vamplog", namespace), instance.Metadata["name"])
	assert.Equal(t, "istio-system", instance.Metadata["namespace"])
	_, marshalError := yaml.Marshal(instance)
	assert.Equal(t, nil, marshalError)
}
