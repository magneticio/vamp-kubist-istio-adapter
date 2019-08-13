package metriclogger_test

import (
	"testing"

	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/metriclogger"
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
