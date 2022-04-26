package storage

import (
	"sync"
)

type MemStorage struct {
	gauges               map[string]float64
	counters             map[string]int64
	gaugesMutexMap       map[string]*sync.Mutex
	gaugeMutexMapMutex   sync.Mutex
	counterMutexMap      map[string]*sync.Mutex
	counterMutexMapMutex sync.Mutex
}

func (storage *MemStorage) IncrementCounter(key string, value int64) error {
	mu, found := storage.counterMutexMap[key]
	if !found {
		storage.counterMutexMapMutex.Lock()
		// проверяем, что мьютекс не был создан, пока мы
		// брали мьютекс на мапу
		mu, found = storage.counterMutexMap[key]
		if !found {
			mu = new(sync.Mutex)
			storage.counterMutexMap[key] = mu
		}
		storage.counterMutexMapMutex.Unlock()
	}
	mu.Lock()
	prev, found := storage.counters[key]
	n := value
	if found {
		n += prev
	}
	storage.counters[key] = n
	mu.Unlock()
	return nil
}

func (storage *MemStorage) GetCounter(key string) (int64, bool, error) {
	value, found := storage.counters[key]
	if !found {
		return 0, false, nil
	} else {
		return value, true, nil
	}
}

func (storage *MemStorage) SetGauge(key string, value float64) error {
	mu, found := storage.gaugesMutexMap[key]
	if !found {
		storage.gaugeMutexMapMutex.Lock()
		// проверяем, что мьютекс не был создан, пока мы
		// брали мьютекс на мапу
		mu, found = storage.gaugesMutexMap[key]
		if !found {
			mu = new(sync.Mutex)
			storage.gaugesMutexMap[key] = mu
		}
		storage.gaugeMutexMapMutex.Unlock()
	}
	mu.Lock()
	storage.gauges[key] = value
	mu.Unlock()
	return nil
}

func (storage *MemStorage) GetGauge(key string) (float64, bool, error) {
	value, found := storage.gauges[key]
	if !found {
		return 0, false, nil
	} else {
		return value, true, nil
	}
}

func NewMemStorage() *MemStorage {
	m := new(MemStorage)
	m.counterMutexMap = map[string]*sync.Mutex{}
	m.gaugesMutexMap = map[string]*sync.Mutex{}
	m.counters = map[string]int64{}
	m.gauges = map[string]float64{}
	return m
}
