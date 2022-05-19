package schema

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestMetrics_Explain(t *testing.T) {

	params := []struct {
		m        Metrics
		expected [3]string
	}{
		{NewCounter("cntID", 42), [...]string{"cntID", "counter", "42"}},
		{NewGauge("ggID", 13.37), [...]string{"ggID", "gauge", "13.37"}},
		{Metrics{"ID", "type", nil, nil}, [...]string{"ID", "type", "(nil)"}},
	}

	for _, param := range params {
		m := param.m
		id, tp, val := m.Explain()
		actual := [...]string{id, tp, val}
		assert.Equal(t, param.expected, actual)
	}
}
