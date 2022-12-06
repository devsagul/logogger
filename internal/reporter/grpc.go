package reporter

import (
	"context"
	"log"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"logogger/internal/crypt"
	"logogger/internal/proto"
	"logogger/internal/schema"
)

type GRPCReporter struct {
	encryptor crypt.Encryptor
	ip        string
	wg        sync.WaitGroup
}

func (r *GRPCReporter) ReportMetricsBatches(ctx context.Context, l []schema.Metrics, host string) error {
	conn, err := grpc.Dial(host, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		e := conn.Close()
		if e != nil {
			log.Printf("Error while closing grpc connection: %v", e)
		}
	}()
	c := proto.NewLogoggerClient(conn)

	md := metadata.New(map[string]string{"X-Real-IP": r.ip})
	ctx = metadata.NewOutgoingContext(ctx, md)

	var metrics []*proto.MetricsValue
	for _, m := range l {
		metrics = append(metrics, &proto.MetricsValue{
			Id:    m.ID,
			Type:  m.MType,
			Delta: m.Delta,
			Value: m.Value,
			Hash:  m.Hash,
		})
	}
	in := &proto.MetricsList{Metrics: metrics}
	_, err = c.UpdateValues(ctx, in)
	return err
}

func (r *GRPCReporter) Shutdown() {
	r.wg.Wait()
}

func NewGRPCReporter(encryptor crypt.Encryptor) (*GRPCReporter, error) {
	res := new(GRPCReporter)
	res.encryptor = encryptor
	ip, err := getRealIP()
	if err != nil {
		return nil, err
	}
	res.ip = ip
	res.wg = sync.WaitGroup{}
	return res, nil
}
