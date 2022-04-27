package storage

type MetricDef struct {
	Type  string
	Name  string
	Value interface{}
}

type MetricsStorage interface {
	Increment(key string, value interface{}) error
	Get(key string) (value MetricDef, found bool, err error)
	Put(key string, value MetricDef) error
	List() ([]MetricDef, error)
}
