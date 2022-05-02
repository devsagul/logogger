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

func writeError(w http.ResponseWriter, e error) {
	switch err := e.(type) {
	case nil:
	case *requestError:
		safeWrite(w, err.status, err.body)
	case *validationError:
		safeWrite(w, http.StatusBadRequest, err.Error())
	case *storage.NotFound:
		safeWrite(w, http.StatusNotFound, "Could not find metrics with name %s", err.ID)
	case *storage.IncrementingNonCounterMetrics:
		safeWrite(w, http.StatusNotImplemented, err.ActualType)
	case *storage.TypeMismatch:
		safeWrite(w, http.StatusConflict, fmt.Sprintf("Requested operation on metrics %s with type %s, but actual type in storage is %s", err.ID, err.Requested, err.Stored))
	default:
		safeWrite(w, http.StatusInternalServerError, "Internal Server Error")
	}
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
		return schema.NewEmptyMetrics(), &requestError{http.StatusNotImplemented, fmt.Sprintf("Could not perform requested operation on type %s", valueType)}
	}
}
