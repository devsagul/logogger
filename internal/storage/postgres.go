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
	db  *sql.DB
	ctx context.Context
}

func (p *PostgresStorage) Put(req schema.Metrics) error {
	switch req.MType {
	case schema.MetricsTypeCounter:
		log.Println("prepare query")
		putQuery, err := p.db.PrepareContext(p.ctx, "INSERT INTO metric(id, type, delta, value) VALUES($1, 'counter', $2, NULL) ON CONFLICT (id) DO UPDATE SET type='counter', delta=EXCLUDED.delta, value=NULL")
		if err != nil {
			return err
		}
		log.Println("exec query")
		_, err = putQuery.ExecContext(p.ctx, req.ID, *req.Delta)
		if err != nil {
			return err
		}
	case schema.MetricsTypeGauge:
		log.Println("prepare query")
		putQuery, err := p.db.PrepareContext(p.ctx, "INSERT INTO metric(id, type, delta, value) VALUES($1, 'gauge', NULL, $2) ON CONFLICT (id) DO UPDATE SET type='gauge', delta=NULL, value=EXCLUDED.value")
		if err != nil {
			log.Printf("PREPARE ERROR: %v", err)
			return err
		}
		log.Println("exec query")
		_, err = putQuery.ExecContext(p.ctx, req.ID, *req.Value)
		if err != nil {
			log.Printf("EXEC ERROR: %v", err)
			return err
		}
	default:
		return fmt.Errorf("unsupported metrics type: %s", req.MType)
	}
	return nil
}

func (p *PostgresStorage) Extract(req schema.Metrics) (schema.Metrics, error) {
	res := schema.NewEmptyMetrics()

	extractQuery, err := p.db.PrepareContext(p.ctx, "SELECT type, delta, value FROM metric WHERE id = $1")
	if err != nil {
		return schema.NewEmptyMetrics(), err
	}
	row := extractQuery.QueryRowContext(p.ctx, req.ID)
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
	} else {
		res.Delta = nil
	}
	if value.Valid {
		res.Value = &value.Float64
	} else {
		res.Value = nil
	}
	return res, nil
}

func (p *PostgresStorage) Increment(req schema.Metrics, value int64) error {
	if req.MType != "counter" {
		return incrementingNonCounterMetrics(req.ID, req.MType)
	}

	tx, err := p.db.BeginTx(p.ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		err = tx.Rollback()
		if err != nil {
			log.Printf("Error occured on Rollback: %s", err.Error())
		}
	}()

	_, err = p.Extract(req)
	if err != nil {
		return err
	}

	incrementQuery, err := p.db.PrepareContext(p.ctx, "UPDATE metric SET delta = delta + $2 WHERE id = $1")
	if err != nil {
		return err
	}
	_, err = incrementQuery.ExecContext(p.ctx, req.ID, value)
	if err != nil {
		return err
	}

	err = tx.Commit()
	return err
}

func (p *PostgresStorage) List() ([]schema.Metrics, error) {
	var res []schema.Metrics

	tx, err := p.db.BeginTx(p.ctx, nil)
	defer func() {
		err = tx.Rollback()
		if err != nil {
			log.Printf("Error occured on Rollback: %s", err.Error())
		}
	}()

	if err != nil {
		return res, err
	}
	query, err := tx.PrepareContext(p.ctx, "SELECT id, type, delta, value FROM metric")
	if err != nil {
		return res, err
	}
	rows, err := query.QueryContext(p.ctx)
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

func (p *PostgresStorage) BulkPut(values []schema.Metrics) error {
	tx, err := p.db.BeginTx(p.ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		err = tx.Rollback()
		if err != nil {
			log.Printf("Error occured on Rollback: %s", err.Error())
		}
	}()
	putQuery, err := p.db.PrepareContext(p.ctx, "INSERT INTO metric(id, type, delta, value) VALUES($1, $2, $3, $4)")
	if err != nil {
		return err
	}
	for _, metric := range values {
		switch metric.MType {
		case schema.MetricsTypeCounter:
			_, err = putQuery.ExecContext(p.ctx, metric.ID, metric.MType, *metric.Delta, nil)
			if err != nil {
				return err
			}
		case schema.MetricsTypeGauge:
			_, err = putQuery.ExecContext(p.ctx, metric.ID, metric.MType, nil, *metric.Value)
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported metrics type: %s", metric.MType)
		}
	}

	err = tx.Commit()
	return err
}

func (p *PostgresStorage) BulkUpdate(counters []schema.Metrics, gauges []schema.Metrics) error {
	tx, err := p.db.BeginTx(p.ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		err = tx.Rollback()
		if err != nil {
			log.Printf("Error occured on Rollback: %s", err.Error())
		}
	}()

	putQuery, err := p.db.PrepareContext(p.ctx, "INSERT INTO metric(id, type, delta, value) VALUES($1, 'counter', $2, NULL) ON CONFLICT (id) DO UPDATE SET type='counter', delta=metric.delta+EXCLUDED.delta, value=NULL")
	if err != nil {
		return err
	}
	for _, m := range counters {
		_, err = putQuery.ExecContext(p.ctx, m.ID, *m.Delta)
		if err != nil {
			return err
		}
	}

	putQuery, err = p.db.PrepareContext(p.ctx, "INSERT INTO metric(id, type, delta, value) VALUES($1, 'gauge', NULL, $2) ON CONFLICT (id) DO UPDATE SET type='gauge', delta=NULL, value=EXCLUDED.value")
	if err != nil {
		return err
	}
	for _, m := range gauges {
		_, err = putQuery.ExecContext(p.ctx, m.ID, *m.Value)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (p *PostgresStorage) Ping() error {
	return p.db.PingContext(p.ctx)
}

func (p *PostgresStorage) Close() error {
	return p.db.Close()
}

func (p *PostgresStorage) WithContext(ctx context.Context) MetricsStorage {
	newStorage := *p
	newStorage.ctx = ctx
	return &newStorage
}

func NewPostgresStorage(dsn string) (*PostgresStorage, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS metric (id VARCHAR(255) PRIMARY KEY, type VARCHAR(255) NOT NULL, delta BIGINT, value DOUBLE PRECISION)")
	if err != nil {
		return nil, err
	}

	p := new(PostgresStorage)
	p.db = db
	p.ctx = context.Background()
	return p, nil
}
