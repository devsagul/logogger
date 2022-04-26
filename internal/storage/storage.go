package storage

type MetricsStorage interface {
	IncrementCounter(key string, value int64) error
	GetCounter(key string) (value int64, found bool, err error)
	SetGauge(key string, value float64) error
	GetGauge(key string) (value float64, found bool, err error)
}
