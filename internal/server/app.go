package server

import (
	"context"
	"fmt"
	"log"
	"time"

	"logogger/internal/dumper"
	"logogger/internal/schema"
	"logogger/internal/storage"
)

type App struct {
	dumper dumper.Dumper
	store  storage.MetricsStorage
	key    string
	sync   bool
}

func NewApp(
	store storage.MetricsStorage,
) *App {
	app := new(App)
	app.store = store
	app.sync = false
	app.key = ""
	app.dumper = dumper.NoOpDumper{}
	return app
}

func (app *App) retrieveValue(
	ctx context.Context,
	req schema.Metrics,
) (
	schema.Metrics,
	*applicationError,
) {

	switch req.MType {
	case schema.MetricsTypeCounter:
	case schema.MetricsTypeGauge:
	default:
		return schema.NewEmptyMetrics(), convertError(
			ValidationError(
				fmt.Sprintf(
					"Unable to perform requested action on metrics type %s", req.MType,
				),
			),
		)
	}

	value, err := app.store.Extract(ctx, req)
	if err != nil {
		return schema.NewEmptyMetrics(), convertError(err)
	}

	if app.key != "" {
		if err = value.Sign(app.key); err != nil {
			return schema.NewEmptyMetrics(), convertError(err)
		}
	}
	return value, nil
}

func (app *App) updateValue(
	ctx context.Context,
	req schema.Metrics,
) (
	schema.Metrics,
	*applicationError,
) {
	var err error

	if (req.MType == schema.MetricsTypeCounter && req.Delta == nil) || (req.MType == schema.MetricsTypeGauge && req.Value == nil) {
		return schema.NewEmptyMetrics(), convertError(
			ValidationError(
				"missing value",
			),
		)
	}

	if app.key != "" {
		signed, e := req.IsSignedWithKey(app.key)
		if e != nil {
			return schema.NewEmptyMetrics(), convertError(e)
		}
		if !signed {
			return schema.NewEmptyMetrics(), convertError(
				ValidationError(
					"signature mismatch",
				),
			)
		}
	}

	switch req.MType {
	case schema.MetricsTypeCounter:
		err = app.store.Increment(ctx, req, *req.Delta)
		switch err.(type) {
		case *storage.NotFound:
			err = app.store.Put(ctx, req)
		}
	case schema.MetricsTypeGauge:
		err = app.store.Put(ctx, req)
	default:
		return schema.NewEmptyMetrics(), convertError(ValidationError(
			fmt.Sprintf(
				"Unable to perform requested action on metrics type %s", req.MType,
			),
		))
	}

	if err != nil {
		return schema.NewEmptyMetrics(), convertError(err)
	}

	if app.sync {
		// by the logic of it, we should not depend on the request's context
		// while syncing our state with dumper, so we use background context
		go app.safeDump(context.Background())
	}

	return app.retrieveValue(ctx, req)
}

func (app *App) listValues(ctx context.Context) (
	[]schema.Metrics,
	*applicationError,
) {
	values, err := app.store.List(ctx)
	if app.key != "" {
		for _, value := range values {
			if app.key != "" {
				if err = value.Sign(app.key); err != nil {
					return nil, convertError(err)
				}
			}
		}
	}
	return values, convertError(err)
}

func (app *App) bulkUpdateValues(ctx context.Context, values []schema.Metrics) (
	[]schema.Metrics,
	*applicationError,
) {
	var gauges []schema.Metrics
	var counters []schema.Metrics

	for _, value := range values {
		if app.key != "" {
			signed, err := value.IsSignedWithKey(app.key)
			if err != nil {
				return nil, convertError(err)
			}
			if !signed {
				return nil, convertError(ValidationError("signature mismatch"))
			}
		}
		if (value.MType == schema.MetricsTypeCounter && value.Delta == nil) || (value.MType == schema.MetricsTypeGauge && value.Value == nil) {
			return nil, convertError(
				ValidationError(
					"missing value",
				),
			)
		}

		switch value.MType {
		case schema.MetricsTypeCounter:
			counters = append(counters, value)
		case schema.MetricsTypeGauge:
			gauges = append(gauges, value)
		default:
			return nil, convertError(ValidationError(
				fmt.Sprintf(
					"Unable to perform requested action on metrics type %s", value.MType,
				),
			))
		}
	}
	err := app.store.BulkUpdate(ctx, counters, gauges)
	if err != nil {
		return nil, convertError(err)
	}

	if app.sync {
		// by the logic of it, we should not depend on the request's context
		// while syncing our state with dumper, so we use background context
		go app.safeDump(context.Background())
	}

	return app.listValues(ctx)
}

func (app *App) ping(ctx context.Context) *applicationError {
	err := app.store.Ping(ctx)
	log.Printf("Ping result: %v", err)
	return convertError(err)
}

func (app *App) safeDump(ctx context.Context) {
	log.Print("Dumping current storage state...")
	l, err := app.store.List(ctx)
	if err != nil {
		log.Print("Could not retrieve values from storage")
		return
	}

	err = app.dumper.Dump(l)
	if err != nil {
		log.Print("Could not write storage data")
	}
}

func (app *App) WithDumper(d dumper.Dumper) *App {
	app.dumper = d
	return app
}

func (app *App) WithDumpInterval(interval time.Duration) *App {
	if interval == 0 {
		app.sync = true
		return app
	}

	app.sync = false
	t := time.NewTicker(interval)
	go func() {
		for {
			<-t.C
			app.safeDump(context.Background())
		}
	}()

	return app
}

func (app *App) WithKey(key string) *App {
	app.key = key
	return app
}
