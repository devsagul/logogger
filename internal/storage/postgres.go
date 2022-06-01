package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"logogger/internal/schema"

	_ "github.com/lib/pq"
)

type PostgresStorage struct {
	db *sql.DB
}

func (p *PostgresStorage) Put(req schema.Metrics) error {
	switch req.MType {
	case "counter":
		log.Println("prepare query")
		putQuery, err := p.db.Prepare("INSERT INTO metric(id, type, delta, value) VALUES(?, 'counter', ?, NULL) ON CONFLICT DO UPDATE SET type='counter', delta=EXCLUDED.delta, value=NULL")
		if err != nil {
			return err
		}
		log.Println("exec query")
		_, err = putQuery.Exec(req.ID, *req.Delta)
		if err != nil {
			return err
		}
	case "gauge":
		log.Println("prepare query")
		putQuery, err := p.db.Prepare("INSERT INTO metric(id, type, delta, value) VALUES(?, 'gauge', NULL, ?) ON CONFLICT DO UPDATE SET type='gauge', delta=NULL, value=EXCLUDED.value")
		if err != nil {
			return err
		}
		log.Println("exec query")
		_, err = putQuery.Exec(req.ID, *req.Value)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported metrics type: %s", req.MType)
	}
	return nil
}

func (p *PostgresStorage) Extract(req schema.Metrics) (schema.Metrics, error) {
	res := schema.NewEmptyMetrics()

	extractQuery, err := p.db.Prepare("SELECT type, delta, value FROM metric WHERE id = ?")
	if err != nil {
		return schema.NewEmptyMetrics(), err
	}
	row := extractQuery.QueryRow(req.ID)
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

	tx, err := p.db.Begin()
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

	incrementQuery, err := p.db.Prepare("UPDATE metric SET delta = delta + ? WHERE id = ?")
	if err != nil {
		return err
	}
	_, err = incrementQuery.Exec(req.ID, value)
	if err != nil {
		return err
	}

	err = tx.Commit()
	return err
}

func (p *PostgresStorage) List() ([]schema.Metrics, error) {
	var res []schema.Metrics

	tx, err := p.db.Begin()
	defer func() {
		err := tx.Rollback()
		if err != nil {
			log.Printf("Error occured on Rollback: %s", err.Error())
		}
	}()

	if err != nil {
		return res, err
	}
	query, err := tx.Prepare("SELECT id, type, delta, value FROM metric")
	if err != nil {
		return res, err
	}
	rows, err := query.Query()
	if err != nil {
		return res, err
	}
	for rows.Next() {
		var row schema.Metrics
		err = rows.Scan(&row.ID, &row.MType, row.Delta, row.Value)
		if err != nil {
			return res, err
		}
		res = append(res, row)
	}
	err = rows.Err()
	return res, err
}

func (p *PostgresStorage) BulkPut(values []schema.Metrics) error {
	tx, err := p.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		err := tx.Rollback()
		if err != nil {
			log.Printf("Error occured on Rollback: %s", err.Error())
		}
	}()
	putQuery, err := p.db.Prepare("INSERT INTO metric(id, type, delta, value) VALUES(?, ?, ?, ?)")
	if err != nil {
		return err
	}
	for _, metric := range values {
		switch metric.MType {
		case "counter":
			_, err = putQuery.Exec(metric.ID, metric.MType, *metric.Delta, nil)
			if err != nil {
				return err
			}
		case "gauge":
			_, err = putQuery.Exec(metric.ID, metric.MType, nil, *metric.Value)
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

func (p *PostgresStorage) Ping() error {
	return p.db.Ping()
}

func (p *PostgresStorage) Close() error {
	return p.db.Close()
}

func NewPostgresStorage(dsn string) (*PostgresStorage, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS metric (id VARCHAR(255) PRIMARY KEY, type VARCHAR(255) NOT NULL, delta INTEGER, value DOUBLE PRECISION)")
	if err != nil {
		return nil, err
	}

	p := new(PostgresStorage)
	p.db = db
	return p, nil
}
