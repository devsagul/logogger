package schema

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
)

var delta int64 = 42
var value = 13.37

var serializationTests = []struct {
	in  Metrics
	out string
}{
	{Metrics{ID: "test", MType: "generic"}, `{"id": "test", "type": "generic"}`},
	{Metrics{ID: "poll", MType: "counter", Delta: &delta}, `{"id": "poll", "type": "counter", "delta": 42}`},
	{Metrics{ID: "load", MType: "gauge", Value: &value}, `{"id": "load", "type": "gauge", "value": 13.37}`},
}

func TestMetrics_SerializeRequest(t *testing.T) {
	for _, data := range serializationTests {
		serialized, err := json.Marshal(data.in)
		assert.Nil(t, err)
		assert.JSONEq(t, data.out, string(serialized))
	}
}
