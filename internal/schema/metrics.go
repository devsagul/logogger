package schema

import (
	"fmt"
	"strconv"
)

type Metrics struct {
	ID    string   `json:"id"`
	MType string   `json:"type"`
	Delta *int64   `json:"delta,omitempty"`
	Value *float64 `json:"value,omitempty"`
}

func NewEmptyMetrics() Metrics {
	return Metrics{"", "", nil, nil}
}

func NewCounterRequest(id string) Metrics {
	return Metrics{ID: id, MType: "counter"}
}

func NewGaugeRequest(id string) Metrics {
	return Metrics{ID: id, MType: "gauge"}
}

func NewCounter(id string, delta int64) Metrics {
	return Metrics{ID: id, MType: "counter", Delta: &delta}
}

func NewGauge(id string, value float64) Metrics {
	return Metrics{ID: id, MType: "gauge", Value: &value}
}

func (m Metrics) Explain() (string, string, string) {
	value := "(nil)"
	switch m.MType {
	case "counter":
		if m.Delta != nil {
			value = fmt.Sprintf("%d", *m.Delta)
		}
	case "gauge":
		if m.Value != nil {
			value = strconv.FormatFloat(*m.Value, 'f', -1, 64)
		}
	default:
	}
	return m.ID, m.MType, value
}
