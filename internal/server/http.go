package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"io"
	"log"
	"logogger/internal/crypt"
	"logogger/internal/schema"
	"net"
	"net/http"
	"strings"
	"time"
)

type HttpServer struct {
	app           *App
	decryptor     crypt.Decryptor
	trustedSubnet *net.IPNet
	Router        chi.Router
}

type serverHandler func(w http.ResponseWriter, r *http.Request) *applicationError

func makeHandler(handler serverHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		errChan := make(chan *applicationError, 1)
		ctx := r.Context()
		go func() {
			errChan <- handler(w, r)
		}()

		select {
		case err := <-errChan:
			if err != nil {
				SafeWrite(w, err.httpStatus, err.wrapped.Error())
			}
		case <-ctx.Done():
			return
		}
	}
}

func (server *HttpServer) retrieveValue(
	w http.ResponseWriter,
	r *http.Request,
) *applicationError {
	valueType := chi.URLParam(r, "Type")
	name := chi.URLParam(r, "Name")

	req := schema.NewEmptyMetrics()
	req.MType = valueType
	req.ID = name

	value, err := server.app.retrieveValue(r.Context(), req)
	if err != nil {
		return err
	}
	_, _, body := value.Explain()
	SafeWrite(w, http.StatusOK, body)
	return nil
}

func (server *HttpServer) updateValue(w http.ResponseWriter, r *http.Request) *applicationError {
	valueType := chi.URLParam(r, "Type")
	name := chi.URLParam(r, "Name")
	rawValue := chi.URLParam(r, "Value")

	value, err := ParseMetric(valueType, name, rawValue)
	if err != nil {
		return convertError(err)
	}

	_, appErr := server.app.updateValue(r.Context(), value)
	if appErr != nil {
		return appErr
	}
	SafeWrite(w, http.StatusOK, "Status: OK")
	return nil
}

func (server *HttpServer) listValues(w http.ResponseWriter, r *http.Request) *applicationError {
	list, err := server.app.listValues(r.Context())
	if err != nil {
		return err
	}

	if len(list) == 0 {
		w.WriteHeader(http.StatusOK)
		return nil
	}
	var sb strings.Builder

	header := "<table><tr><th>Type</th><th>Name</th><th>Value</th></tr>"
	sb.Write([]byte(header))
	for _, metrics := range list {
		name, mType, value := metrics.Explain()
		row := fmt.Sprintf("<tr><td>%s</td><td>%s</td><td>%s</td></tr>", name, mType, value)
		sb.Write([]byte(row))
	}
	footer := "</table>"
	sb.Write([]byte(footer))
	SafeWrite(w, http.StatusOK, sb.String())
	return nil
}

func (server *HttpServer) updateValueJSON(w http.ResponseWriter, r *http.Request) *applicationError {
	if r.Body == nil {
		return convertError(
			ValidationError(
				"empty body",
			),
		)
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		return convertError(err)
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var m schema.Metrics
	err = decoder.Decode(&m)
	if err != nil {
		return convertError(
			ValidationError(
				err.Error(),
			),
		)
	}

	value, appErr := server.app.updateValue(r.Context(), m)
	if appErr != nil {
		return appErr
	}

	serialized, err := json.Marshal(value)
	if err != nil {
		return convertError(err)
	}

	SafeWrite(w, http.StatusOK, string(serialized))
	return nil
}

func (server *HttpServer) updateValuesJSON(w http.ResponseWriter, r *http.Request) *applicationError {
	if r.Body == nil {
		return convertError(ValidationError("empty body"))
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		return convertError(err)
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var m []schema.Metrics
	err = decoder.Decode(&m)
	if err != nil {
		return convertError(ValidationError(err.Error()))
	}

	values, e := server.app.bulkUpdateValues(r.Context(), m)
	if e != nil {
		return e
	}

	if len(values) == 0 {
		w.WriteHeader(http.StatusOK)
		return nil
	}

	value := values[0]
	serialized, err := json.Marshal(value)
	if err != nil {
		return convertError(err)
	}
	SafeWrite(w, http.StatusOK, string(serialized))
	return nil
}

func (server *HttpServer) retrieveValueJSON(w http.ResponseWriter, r *http.Request) *applicationError {
	if r.Body == nil {
		return convertError(ValidationError("empty body"))
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		return convertError(ValidationError(err.Error()))
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	var m schema.Metrics
	err = decoder.Decode(&m)
	if err != nil {
		return convertError(ValidationError(err.Error()))
	}

	value, e := server.app.retrieveValue(r.Context(), m)
	if e != nil {
		return e
	}

	serialized, err := json.Marshal(value)
	if err != nil {
		return convertError(err)
	}

	SafeWrite(w, http.StatusOK, string(serialized))
	return nil
}

func (server *HttpServer) ping(w http.ResponseWriter, r *http.Request) *applicationError {
	err := server.app.ping(r.Context())
	log.Printf("Ping result: %v", err)
	if err != nil {
		return err
	}
	SafeWrite(w, http.StatusOK, "Status: OK")
	return nil
}

func NewHttpServer(app *App) *HttpServer {
	server := new(HttpServer)
	server.app = app
	server.decryptor = crypt.NoOpDecryptor{}
	server.trustedSubnet = nil

	r := chi.NewRouter()
	// useful middlewares
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(middleware.Compress(5))

	r.Use(server.trustedSubnetMiddleware())
	r.Use(server.decryptionMiddleware())

	r.With(middleware.SetHeader("Content-Type", "text/plain")).Get("/value/{Type}/{Name}", makeHandler(server.retrieveValue))
	r.With(middleware.SetHeader("Content-Type", "text/plain")).Post("/update/{Type}/{Name}/{Value}", makeHandler(server.updateValue))
	r.With(middleware.SetHeader("Content-Type", "text/html")).Get("/", makeHandler(server.listValues))
	r.With(middleware.SetHeader("Content-Type", "application/json")).Post("/update/", makeHandler(server.updateValueJSON))
	r.With(middleware.SetHeader("Content-Type", "application/json")).Post("/updates/", makeHandler(server.updateValuesJSON))
	r.With(middleware.SetHeader("Content-Type", "application/json")).Post("/value/", makeHandler(server.retrieveValueJSON))
	r.With(middleware.SetHeader("Content-Type", "text/plain")).Get("/ping", makeHandler(server.ping))

	server.Router = r
	return server
}

func (server *HttpServer) WithDecryptor(decryptor crypt.Decryptor) *HttpServer {
	server.decryptor = decryptor
	return server
}

func (server *HttpServer) WithTrustedSubnet(trustedSubnet *net.IPNet) *HttpServer {
	server.trustedSubnet = trustedSubnet
	return server
}

func (server *HttpServer) decryptionMiddleware() func(handler http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				data, err := io.ReadAll(r.Body)
				if err != nil {
					log.Printf("ERROR: %+v", err)
					w.WriteHeader(http.StatusInternalServerError)
					return
				}

				if data != nil {
					data, err = server.decryptor.Decrypt(data)
					if err != nil {
						log.Printf("ERROR: %+v", err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}

					r.Body = io.NopCloser(bytes.NewReader(data))
					r.ContentLength = int64(len(data))
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (server *HttpServer) trustedSubnetMiddleware() func(handler http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if server.trustedSubnet == nil {
				next.ServeHTTP(w, r)
				return
			}

			rawIP := r.Header.Get("X-Real-IP")
			ip := net.ParseIP(rawIP)
			if ip == nil {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			if !server.trustedSubnet.Contains(ip) {
				w.WriteHeader(http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
