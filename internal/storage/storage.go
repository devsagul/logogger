package storage

import (
	"logogger/internal/poller"
)

type MetricsStorage interface {
	Write(metrics poller.Metrics) error
	Read() (poller.Metrics, error)
}
