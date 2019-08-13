package subsetmapper_test

import (
	"fmt"
	"testing"

	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/subsetmapper"
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
