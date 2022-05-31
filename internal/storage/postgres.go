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
		putQuery, err := p.db.Prepare("DELETE FROM metric WHERE id = ?; INSERT INTO metric(id, type, delta, value) VALUES(?, counter, ?, NULL)")
		if err != nil {
			return err
		}
		_, err = putQuery.Exec(req.ID, *req.Delta)
		if err != nil {
			return err
		}
	case "gauge":
		putQuery, err := p.db.Prepare("DELETE FROM metric WHERE id = ?; INSERT INTO gauge(id, type, delta, value) VALUES(?, gauge, NULL, ?)")
		if err != nil {
			return err
		}
		_, err = putQuery.Exec(req.ID, req.ID, *req.Value)
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
	err = row.Scan(&res.MType, res.Delta, res.Value)
	if err != nil {
		if errors.As(err, &sql.ErrNoRows) {
			return schema.NewEmptyMetrics(), notFound(req.ID)
		}
		return schema.NewEmptyMetrics(), err
	}
	if res.MType != req.MType {
		return schema.NewEmptyMetrics(), typeMismatch(req.ID, req.MType, res.MType)
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
	putQuery, err := p.db.Prepare("INSERT INTO metric(id, type, delta, value) VALUES(?, ?, ?, ?) ON CONFLICT DO UPDATE")
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
