package storage

import (
	"fmt"
)

type NotFound struct {
	wrapped error
}

func (err NotFound) Error() string {
	return err.wrapped.Error()
}

func notFound(key string) NotFound {
	return NotFound{fmt.Errorf("metrics with key %s not found in the storage", key)}
}

type IncrementingNonCounterMetrics struct {
	wrapped error
}

func (err IncrementingNonCounterMetrics) Error() string {
	return err.wrapped.Error()
}

func incrementingNonCounterMetrics(key string, actualType string) IncrementingNonCounterMetrics {
	return IncrementingNonCounterMetrics{
		fmt.Errorf("could not increment value %s, currently it's holding value of type %s", key, actualType),
	}
}

type TypeMismatch struct {
	wrapped error
}

func (err TypeMismatch) Error() string {
	return err.wrapped.Error()
}

func typeMismatch(expectedType string, actualType string) TypeMismatch {
	return TypeMismatch{fmt.Errorf("expected value of type %s but got %s", expectedType, actualType)}
}
