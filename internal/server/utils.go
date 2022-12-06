package server

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"logogger/internal/schema"
)

func SafeWrite(w http.ResponseWriter, status int, format string, args ...interface{}) {
	w.WriteHeader(status)
	body := fmt.Sprintf(format, args...)
	if w.Header().Get("Content-Type") == "application/json" {
		if status >= 300 {
			body = fmt.Sprintf(`{error: "%s"}`, body)
		}
	}
	_, err := w.Write([]byte(body))
	if err != nil {
		log.Printf("Error: could not write response. Cause: %s", err)
	}
}

func ParseMetric(valueType string, name string, rawValue string) (schema.Metrics, error) {
	switch valueType {
	case schema.MetricsTypeCounter:
		value, err := strconv.ParseInt(rawValue, 10, 64)
		if err != nil {
			return schema.NewEmptyMetrics(), ValidationError(fmt.Sprintf("Could not parse int from %s", rawValue))
		}
		return schema.NewCounter(name, value), nil
	case schema.MetricsTypeGauge:
		value, err := strconv.ParseFloat(rawValue, 64)
		if err != nil {
			return schema.NewEmptyMetrics(), ValidationError(fmt.Sprintf("Could not parse float from %s", rawValue))
		}
		return schema.NewGauge(name, value), nil
	default:
		return schema.NewEmptyMetrics(), InvalidTypeError(
			fmt.Sprintf(
				"Unable to perform requested action on metrics type %s", valueType,
			),
		)
	}
}
