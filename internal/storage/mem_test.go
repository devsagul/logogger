package storage

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMemStorage_GetGaugeAfterSet(t *testing.T) {
	storage := NewMemStorage()
	err := storage.Put("value", MetricDef{"gauge", "value", float64(42)})
	assert.Equal(t, nil, err)
	value, found, err := storage.Get("value")
	assert.Equal(t, nil, err)
	assert.Equal(t, true, found)
	assert.Equal(t, "gauge", value.Type)
	assert.Equal(t, "value", value.Name)
	assert.Equal(t, float64(42), value.Value)
}

func TestMemStorage_GetValueBeforeSet(t *testing.T) {
	storage := NewMemStorage()
	_, found, _ := storage.Get("value")
	assert.Equal(t, false, found)
}

func TestMemStorage_GetCounterAfterSet(t *testing.T) {
	storage := NewMemStorage()
	err := storage.Put("value", MetricDef{"counter", "value", int64(42)})
	assert.Equal(t, nil, err)
	value, found, err := storage.Get("value")
	assert.Equal(t, nil, err)
	assert.Equal(t, true, found)
	assert.Equal(t, "counter", value.Type)
	assert.Equal(t, "value", value.Name)
	assert.Equal(t, int64(42), value.Value)
}

func TestMemStorage_IncrementGauge(t *testing.T) {
	storage := NewMemStorage()
	err := storage.Put("value", MetricDef{"gauge", "value", float64(42)})
	assert.Equal(t, nil, err)
	err = storage.Increment("value", 1)
	assert.Errorf(t, err, "could not increment value value, currently it's holding type gauge")
}
