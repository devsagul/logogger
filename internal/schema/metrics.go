// Package schema defines a structure of messages
package schema

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
)

type Metrics struct {
	ID    string      `json:"id"`
	MType MetricsType `json:"type"`
	Delta *int64      `json:"delta,omitempty"`
	Value *float64    `json:"value,omitempty"`
	Hash  string      `json:"hash,omitempty"`
}

type MetricsType string

const (
	MetricsTypeCounter MetricsType = "counter"
	MetricsTypeGauge   MetricsType = "gauge"
	MetricsTypeEmpty   MetricsType = ""
)

func NewEmptyMetrics() Metrics {
	return Metrics{"", MetricsTypeEmpty, nil, nil, ""}
}

func NewCounterRequest(id string) Metrics {
	return Metrics{ID: id, MType: MetricsTypeCounter}
}

func NewGaugeRequest(id string) Metrics {
	return Metrics{ID: id, MType: MetricsTypeGauge}
}

func NewCounter(id string, delta int64) Metrics {
	return Metrics{ID: id, MType: MetricsTypeCounter, Delta: &delta}
}

func NewGauge(id string, value float64) Metrics {
	return Metrics{ID: id, MType: MetricsTypeGauge, Value: &value}
}

func (m Metrics) Explain() (string, string, string) {
	value := "(nil)"
	switch m.MType {
	case MetricsTypeCounter:
		if m.Delta != nil {
			value = fmt.Sprintf("%d", *m.Delta)
		}
	case MetricsTypeGauge:
		if m.Value != nil {
			value = strconv.FormatFloat(*m.Value, 'f', -1, 64)
		}
	default:
	}
	return m.ID, string(m.MType), value
}

type hashingMetricsError struct {
	reason string
}

func (e hashingMetricsError) Error() string {
	return e.reason
}

func (m Metrics) hash(key string) (string, error) {
	if key == "" {
		return "", hashingMetricsError{"can't sign metrics with empty key"}
	}
	var data string
	switch m.MType {
	case MetricsTypeCounter:
		if m.Delta == nil {
			return "", hashingMetricsError{"cannot sign metrics without value"}
		}
		data = fmt.Sprintf("%s:counter:%d", m.ID, *m.Delta)
	case MetricsTypeGauge:
		if m.Value == nil {
			return "", hashingMetricsError{"cannot sign metrics without value"}
		}
		data = fmt.Sprintf("%s:gauge:%f", m.ID, *m.Value)
	default:
		return "", hashingMetricsError{fmt.Sprintf("unknown metrics type to sign: %s", m.MType)}
	}

	h := hmac.New(sha256.New, []byte(key))
	_, err := h.Write([]byte(data))
	if err != nil {
		return "", err
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum), err
}

func (m *Metrics) Sign(key string) error {
	h, err := m.hash(key)
	if err != nil {
		return err
	}
	m.Hash = h
	return nil
}

func (m Metrics) IsSignedWithKey(key string) (bool, error) {
	h, err := m.hash(key)
	if err == nil {
		return m.Hash == h, nil
	}
	if errors.As(err, &hashingMetricsError{}) {
		return false, nil
	}
	return false, err
}
