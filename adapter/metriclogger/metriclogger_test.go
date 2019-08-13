package metriclogger_test

import (
	"fmt"
	"testing"

	metriclogger "github.com/magneticio/vamp-kubist-istio-adapter/adapter/metriclogger"
	"github.com/montanaflynn/stats"
	"github.com/stretchr/testify/assert"
)

func TestRate(t *testing.T) {
	valuesRaw := []float64{0.1, 0.2, -0.3}
	// 0.1 -0.5 =>  -0.4 => -0.2
	values := stats.LoadRawData(valuesRaw)
	rate, err := metriclogger.Rate(values)
	assert.Equal(t, nil, err)
	assert.Equal(t, float64(-0.2), rate)
}

func TestMapValueToPossibleCodes(t *testing.T) {
	apiProtocol := "http"
	requestMethod := "get"
	responseCode := "200"
	expectedCodes := []string{"", "200", "2xx", "get-200", "get-2xx"}
	codes := metriclogger.MapValueToPossibleCodes(apiProtocol, requestMethod, responseCode)
	fmt.Printf("codes: %v\n", codes)
	assert.Equal(t, expectedCodes, codes)
}

func TestGetMetricLoggerNames(t *testing.T) {
	metricInfo := metriclogger.MetricDefinitions["responseCode"]
	expectedNames := []string{"responseCode", "responseCode-200", "responseCode-2xx", "responseCode-get-200", "responseCode-get-2xx"}
	names := metricInfo.GetMetricLoggerNames("responseCode", "http", "get", "200", int64(200))
	fmt.Printf("e names: %v\n", expectedNames)
	fmt.Printf("names: %v\n", names)
	assert.Equal(t, expectedNames, names)
}
