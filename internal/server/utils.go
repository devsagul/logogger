package server

import (
	"fmt"
	"log"
	"logogger/internal/schema"
	"logogger/internal/storage"
	"net/http"
	"strconv"
)

func SafeWrite(w http.ResponseWriter, status int, format string, args ...interface{}) {
	w.WriteHeader(status)
	body := fmt.Sprintf(format, args...)
	_, err := w.Write([]byte(body))
	if err != nil {
		log.Printf("Error: could not write response. Cause: %s", err)
	}
}

func WriteError(w http.ResponseWriter, e error) {
	var status int
	var error string
	switch err := e.(type) {
	case nil:
	case *requestError:
		status = err.status
		error = err.body
	case *validationError:
		status = http.StatusBadRequest
		error = err.Error()
	case *storage.NotFound:
		status = http.StatusNotFound
		error = fmt.Sprintf("Could not find metrics with name %s", err.ID)
	case *storage.IncrementingNonCounterMetrics:
		status = http.StatusNotImplemented
		error = fmt.Sprintf("Could not increment metrics of type %s", err.ActualType)
	case *storage.TypeMismatch:
		status = http.StatusConflict
		error = fmt.Sprintf("Requested operation on metrics %s with type %s, but actual type in storage is %s", err.ID, err.Requested, err.Stored)
	default:
		status = http.StatusInternalServerError
		error = "Internal Server Error"
	}
	if w.Header().Get("Content-Type") == "application/json" {
		error = fmt.Sprintf(`{error: "%s"}`, error)
	}
	SafeWrite(w, status, error)
}

func ParseMetric(valueType string, name string, rawValue string) (schema.Metrics, error) {
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
