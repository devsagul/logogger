package schema

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
)

var serializationTests = []struct {
	in  Metrics
	out string
}{
	{NewCounterRequest("counterID"), `{"id": "counterID", "type": "counter"}`},
	{NewGaugeRequest("gaugeID"), `{"id": "gaugeID", "type": "gauge"}`},
	{NewCounter("counterID", 42), `{"id": "counterID", "type": "counter", "delta": 42}`},
	{NewGauge("gaugeID", 13.37), `{"id": "gaugeID", "type": "gauge", "value": 13.37}`},
}

func TestMetrics_CreateAndSerialize(t *testing.T) {
	for _, data := range serializationTests {
		serialized, err := json.Marshal(data.in)
		assert.Nil(t, err)
		assert.JSONEq(t, data.out, string(serialized))
	}
}
