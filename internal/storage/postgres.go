package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	_ "github.com/lib/pq"

	"logogger/internal/schema"
)

type PostgresStorage struct {
	db *sql.DB
}

func (p PostgresStorage) Put(ctx context.Context, req schema.Metrics) error {
	var value interface{}
	var query string

	switch req.MType {
	case schema.MetricsTypeCounter:
		query = "INSERT INTO metric(id, type, delta, value) VALUES($1, 'counter', $2, NULL) ON CONFLICT (id) DO UPDATE SET type='counter', delta=EXCLUDED.delta, value=NULL"
		value = *req.Delta
	case schema.MetricsTypeGauge:
		query = "INSERT INTO metric(id, type, delta, value) VALUES($1, 'gauge', NULL, $2) ON CONFLICT (id) DO UPDATE SET type='gauge', delta=NULL, value=EXCLUDED.value"
		value = *req.Value
	default:
		return fmt.Errorf("unsupported metrics type: %s", req.MType)
	}

	putQuery, err := p.db.PrepareContext(ctx, query)
	if err != nil {
		return err
	}
	_, err = putQuery.ExecContext(ctx, req.ID, value)
	return err
}

func (p PostgresStorage) Extract(ctx context.Context, req schema.Metrics) (schema.Metrics, error) {
	res := schema.NewEmptyMetrics()

	extractQuery, err := p.db.PrepareContext(ctx, "SELECT type, delta, value FROM metric WHERE id = $1")
	if err != nil {
		return schema.NewEmptyMetrics(), err
	}
	row := extractQuery.QueryRowContext(ctx, req.ID)
	var delta sql.NullInt64
	var value sql.NullFloat64

	err = row.Scan(&res.MType, &delta, &value)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return schema.NewEmptyMetrics(), notFound(req.ID)
		}
		return schema.NewEmptyMetrics(), err
	}
	if res.MType != req.MType {
		return schema.NewEmptyMetrics(), typeMismatch(req.ID, req.MType, res.MType)
	}
	if delta.Valid {
		res.Delta = &delta.Int64
	}
	if value.Valid {
		res.Value = &value.Float64
	}
	return res, nil
}

func (p PostgresStorage) Increment(ctx context.Context, req schema.Metrics, value int64) error {
	if req.MType != "counter" {
		return incrementingNonCounterMetrics(req.ID, req.MType)
	}

	tx, rollback, err := p.Transaction(ctx)
	if err != nil {
		return err
	}
	defer rollback()

	_, err = p.Extract(ctx, req)
	if err != nil {
		return err
	}

	incrementQuery, err := p.db.PrepareContext(ctx, "UPDATE metric SET delta = delta + $2 WHERE id = $1")
	if err != nil {
		return err
	}
	_, err = incrementQuery.ExecContext(ctx, req.ID, value)
	if err != nil {
		return err
	}

	err = tx.Commit()
	return err
}

func (p PostgresStorage) List(ctx context.Context) ([]schema.Metrics, error) {
	var res []schema.Metrics

	tx, rollback, err := p.Transaction(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback()

	query, err := tx.PrepareContext(ctx, "SELECT id, type, delta, value FROM metric")
	if err != nil {
		return res, err
	}
	rows, err := query.QueryContext(ctx)
	if err != nil {
		return res, err
	}
	for rows.Next() {
		var row schema.Metrics
		var delta sql.NullInt64
		var value sql.NullFloat64
		err = rows.Scan(&row.ID, &row.MType, &delta, &value)
		if delta.Valid {
			row.Delta = &delta.Int64
		}
		if value.Valid {
			row.Value = &value.Float64
		}
		if err != nil {
			return res, err
		}
		res = append(res, row)
	}
	err = rows.Err()
	return res, err
}

func (p PostgresStorage) BulkPut(ctx context.Context, values []schema.Metrics) error {
	tx, rollback, err := p.Transaction(ctx)
	if err != nil {
		return err
	}
	defer rollback()

	putQuery, err := p.db.PrepareContext(ctx, "INSERT INTO metric(id, type, delta, value) VALUES($1, $2, $3, $4)")
	if err != nil {
		return err
	}
	for _, metric := range values {
		switch metric.MType {
		case schema.MetricsTypeCounter:
			_, err = putQuery.ExecContext(ctx, metric.ID, metric.MType, *metric.Delta, nil)
		case schema.MetricsTypeGauge:
			_, err = putQuery.ExecContext(ctx, metric.ID, metric.MType, nil, *metric.Value)
		default:
			return fmt.Errorf("unsupported metrics type: %s", metric.MType)
		}
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (p PostgresStorage) BulkUpdate(ctx context.Context, counters []schema.Metrics, gauges []schema.Metrics) error {
	tx, rollback, err := p.Transaction(ctx)
	if err != nil {
		return err
	}
	defer rollback()

	putQuery, err := p.db.PrepareContext(ctx, "INSERT INTO metric(id, type, delta, value) VALUES($1, 'counter', $2, NULL) ON CONFLICT (id) DO UPDATE SET type='counter', delta=metric.delta+EXCLUDED.delta, value=NULL")
	if err != nil {
		return err
	}
	for _, m := range counters {
		_, err = putQuery.ExecContext(ctx, m.ID, *m.Delta)
		if err != nil {
			return err
		}
	}

	putQuery, err = p.db.PrepareContext(ctx, "INSERT INTO metric(id, type, delta, value) VALUES($1, 'gauge', NULL, $2) ON CONFLICT (id) DO UPDATE SET type='gauge', delta=NULL, value=EXCLUDED.value")
	if err != nil {
		return err
	}
	for _, m := range gauges {
		_, err = putQuery.ExecContext(ctx, m.ID, *m.Value)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (p PostgresStorage) Ping(ctx context.Context) error {
	return p.db.PingContext(ctx)
}

func (p PostgresStorage) Close() error {
	return p.db.Close()
}

func (p PostgresStorage) Transaction(ctx context.Context) (*sql.Tx, func(), error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}

	rollback := func() {
		err = tx.Rollback()
		if err != nil {
			log.Printf("Error occured on Rollback: %s", err.Error())
		}
	}

	return tx, rollback, nil
}

func NewPostgresStorage(dsn string) (PostgresStorage, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return PostgresStorage{}, err
	}
	p := PostgresStorage{db}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS metric (id VARCHAR(255) PRIMARY KEY, type VARCHAR(255) NOT NULL, delta BIGINT, value DOUBLE PRECISION)")
	return p, err
}
