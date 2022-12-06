package server

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"logogger/internal/schema"
)

type okWriter struct {
	status int
}

func (*okWriter) Header() http.Header {
	return map[string][]string{}
}

func (*okWriter) Write([]byte) (int, error) {
	return 0, nil
}

func (w *okWriter) WriteHeader(status int) {
	w.status = status
}

type faultyWriter struct{}

func (faultyWriter) Header() http.Header {
	return map[string][]string{}
}

func (faultyWriter) Write([]byte) (int, error) {
	return 0, errors.New("could not write response")
}

func (faultyWriter) WriteHeader(int) {}

func TestSafeWrite(t *testing.T) {
	// This test asserts, that no unhandled errors propagate outside SafeWrite
	SafeWrite(&okWriter{}, http.StatusOK, "")
	SafeWrite(faultyWriter{}, http.StatusOK, "")
}

func TestParseMetric_ValidCounter(t *testing.T) {
	expected := schema.NewCounter("name", 42)
	actual, err := ParseMetric("counter", "name", "42")

	assert.Equal(t, nil, err)
	assert.Equal(t, expected, actual)
}

func TestParseMetric_InvalidCounter(t *testing.T) {
	_, err := ParseMetric("counter", "name", "13.37")

	assert.IsType(t, &validationError{}, err)
}

func TestParseMetric_ValidGauge(t *testing.T) {
	expected := schema.NewGauge("name", 13.37)
	actual, err := ParseMetric("gauge", "name", "13.37")

	assert.Equal(t, nil, err)
	assert.Equal(t, expected, actual)
}

func TestParseMetric_InvalidGauge(t *testing.T) {
	_, err := ParseMetric("gauge", "name", "leet")

	assert.IsType(t, &validationError{}, err)
}

func TestParseMetric_InvalidGeneric(t *testing.T) {
	_, _ = ParseMetric("generic", "name", "42")

	//assert.IsType(t, &requestError{}, err)
}
