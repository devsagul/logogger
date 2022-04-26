package storage

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMemStorage_GetGaugeAfterSet(t *testing.T) {
	storage := NewMemStorage()
	err := storage.SetGauge("value", 42)
	assert.Equal(t, nil, err)
	value, found, err := storage.GetGauge("value")
	assert.Equal(t, true, found)
	assert.Equal(t, float64(42), value)
}

func TestMemStorage_GetGaugeBeforeSet(t *testing.T) {
	storage := NewMemStorage()
	_, found, _ := storage.GetGauge("value")
	assert.Equal(t, false, found)
}

func TestMemStorage_GetCounterAfterSet(t *testing.T) {
	storage := NewMemStorage()
	err := storage.IncrementCounter("value", 42)
	assert.Equal(t, nil, err)
	value, found, err := storage.GetCounter("value")
	assert.Equal(t, true, found)
	assert.Equal(t, int64(42), value)
}

func TestMemStorage_GetCounterBeforeSet(t *testing.T) {
	storage := NewMemStorage()
	_, found, _ := storage.GetGauge("value")
	assert.Equal(t, false, found)
}
