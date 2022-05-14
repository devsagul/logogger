package schema

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMetrics_CreateAndSerialize(t *testing.T) {
	var serializationTests = []struct {
		in  Metrics
		out string
	}{
		{NewEmptyMetrics(), `{"id": "", "type": ""}`},
		{NewCounterRequest("counterID"), `{"id": "counterID", "type": "counter"}`},
		{NewGaugeRequest("gaugeID"), `{"id": "gaugeID", "type": "gauge"}`},
		{NewCounter("counterID", 42), `{"id": "counterID", "type": "counter", "delta": 42}`},
		{NewGauge("gaugeID", 13.37), `{"id": "gaugeID", "type": "gauge", "value": 13.37}`},
	}
	for _, data := range serializationTests {
		serialized, err := json.Marshal(data.in)
		assert.Nil(t, err)
		assert.JSONEq(t, data.out, string(serialized))
	}
}

// test explain
