package server

import (
	"fmt"
	"log"
	"logogger/internal/schema"
	"logogger/internal/storage"
	"net/http"
	"strconv"
)

func safeWrite(w http.ResponseWriter, status int, format string, args ...interface{}) {
	w.WriteHeader(status)
	body := fmt.Sprintf(format, args...)
	_, err := w.Write([]byte(body))
	if err != nil {
		log.Printf("Error: could not write response. Cause: %s", err)
	}
}

func writeError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}

	reqError, ok := err.(*requestError)
	if ok {
		safeWrite(w, reqError.status, reqError.body)
		return
	}

	validationError, ok := err.(*validationError)
	if ok {
		safeWrite(w, http.StatusBadRequest, validationError.Error())
	}

	notFound, ok := err.(*storage.NotFound)
	if ok {
		safeWrite(w, http.StatusNotFound, "Could not find metrics with name %s", notFound.ID)
		return
	}

	incrementNonCounter, ok := err.(*storage.IncrementingNonCounterMetrics)
	if ok {
		safeWrite(w, http.StatusNotImplemented, incrementNonCounter.ActualType)
		return
	}

	typeMismatch, ok := err.(*storage.TypeMismatch)
	if ok {
		safeWrite(w, http.StatusConflict, fmt.Sprintf("Requested operation on metrics %s with type %s, but actual type in storage is %s", typeMismatch.ID, typeMismatch.Requested, typeMismatch.Stored))
		return
	}

	safeWrite(w, http.StatusInternalServerError, "Internal Server Error")
}

func parseMetric(valueType string, name string, rawValue string) (schema.Metrics, error) {
	switch valueType {
	case "counter":
		value, err := strconv.ParseInt(rawValue, 10, 64)
		if err != nil {
			return schema.NewEmptyMetrics(), ValidationError(fmt.Sprintf("Could not parse int from %s", rawValue))
		}
		return schema.NewCounter(name, value), nil
	case "gauge":
		value, err := strconv.ParseFloat(rawValue, 64)
		if err != nil {
			return schema.NewEmptyMetrics(), ValidationError(fmt.Sprintf("Could not parse float from %s", rawValue))
		}
		return schema.NewGauge(name, value), nil
	default:
		return schema.NewEmptyMetrics(), ValidationError(fmt.Sprintf("Unknown type: %s", valueType))
	}
}
