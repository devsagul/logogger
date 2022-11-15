package storage

import (
	"fmt"

	"logogger/internal/schema"
)

type NotFound struct {
	wrapped error
	ID      string
}

func (err *NotFound) Error() string {
	return err.wrapped.Error()
}

func notFound(key string) *NotFound {
	return &NotFound{fmt.Errorf("metrics with key %s not found in the storage", key), key}
}

type IncrementingNonCounterMetrics struct {
	wrapped    error
	ActualType string
}

func (err *IncrementingNonCounterMetrics) Error() string {
	return err.wrapped.Error()
}

func incrementingNonCounterMetrics(key string, actualType schema.MetricsType) *IncrementingNonCounterMetrics {
	return &IncrementingNonCounterMetrics{
		fmt.Errorf("could not increment value %s, currently it's holding value of type %s", key, actualType), string(actualType),
	}
}

type TypeMismatch struct {
	wrapped   error
	ID        string
	Requested string
	Stored    string
}

func (err *TypeMismatch) Error() string {
	return err.wrapped.Error()
}

func typeMismatch(key string, requestedType schema.MetricsType, storedType schema.MetricsType) *TypeMismatch {
	return &TypeMismatch{fmt.Errorf("expected value of type %s but got %s", requestedType, storedType), key, string(requestedType), string(storedType)}
}
