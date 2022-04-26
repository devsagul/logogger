package storage

import "logogger/internal/poller"

type MemStorage struct {
	metrics poller.Metrics
}

func (storage MemStorage) Write(metrics poller.Metrics) error {
	storage.metrics = metrics
	return nil
}

func (storage MemStorage) Read() (poller.Metrics, error) {
	return storage.metrics, nil
}

func NewMemStorage() MemStorage {
	return *new(MemStorage)
}
