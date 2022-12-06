package server

import (
	"context"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"logogger/internal/crypt"
	"logogger/internal/proto"
	"logogger/internal/schema"
)

type GRPCServer struct {
	proto.UnimplementedLogoggerServer

	app           *App
	decryptor     crypt.Decryptor
	trustedSubnet *net.IPNet
	wrapped       *grpc.Server
}

func (server *GRPCServer) RetrieveValue(ctx context.Context, req *proto.MetricsValue) (*proto.MetricsValue, error) {
	var m schema.Metrics
	m.MType = req.Type
	m.ID = req.Id
	value, err := server.app.retrieveValue(ctx, m)
	if err != nil {
		return nil, status.Errorf(err.grpcStatus, err.wrapped.Error())
	}
	return &proto.MetricsValue{
		Type:  value.MType,
		Id:    value.ID,
		Delta: value.Delta,
		Value: value.Value,
		Hash:  value.Hash,
	}, nil
}

func (server *GRPCServer) ListValues(ctx context.Context, req *proto.Empty) (*proto.MetricsList, error) {
	values, err := server.app.listValues(ctx)
	if err != nil {
		return nil, status.Errorf(err.grpcStatus, err.wrapped.Error())
	}

	var res []*proto.MetricsValue
	for _, value := range values {
		res = append(res, &proto.MetricsValue{
			Type:  value.MType,
			Id:    value.ID,
			Delta: value.Delta,
			Value: value.Value,
			Hash:  value.Hash,
		})
	}

	return &proto.MetricsList{Metrics: res}, nil
}

func (server *GRPCServer) UpdateValue(ctx context.Context, req *proto.MetricsValue) (*proto.MetricsValue, error) {
	var m schema.Metrics
	m.MType = req.Type
	m.ID = req.Id
	m.Value = req.Value
	m.Delta = req.Delta
	m.Hash = req.Hash
	value, err := server.app.updateValue(ctx, m)
	if err != nil {
		return nil, status.Errorf(err.grpcStatus, err.wrapped.Error())
	}
	return &proto.MetricsValue{
		Type:  value.MType,
		Id:    value.ID,
		Delta: value.Delta,
		Value: value.Value,
		Hash:  value.Hash,
	}, nil
}

func (server *GRPCServer) UpdateValues(ctx context.Context, req *proto.MetricsList) (*proto.MetricsList, error) {
	var m []schema.Metrics
	for _, item := range req.Metrics {
		m = append(m, schema.Metrics{
			ID:    item.Id,
			MType: item.Type,
			Delta: item.Delta,
			Value: item.Value,
			Hash:  item.Hash,
		})
	}
	values, err := server.app.bulkUpdateValues(ctx, m)
	if err != nil {
		return nil, status.Errorf(err.grpcStatus, err.wrapped.Error())
	}

	var res []*proto.MetricsValue
	for _, value := range values {
		res = append(res, &proto.MetricsValue{
			Type:  value.MType,
			Id:    value.ID,
			Delta: value.Delta,
			Value: value.Value,
			Hash:  value.Hash,
		})
	}

	return &proto.MetricsList{Metrics: res}, nil
}

func (server *GRPCServer) Ping(ctx context.Context, req *proto.Empty) (*proto.Empty, error) {
	err := server.app.ping(ctx)
	if err != nil {
		return nil, status.Errorf(err.grpcStatus, err.wrapped.Error())
	}
	return nil, nil
}

func (server *GRPCServer) Serve(address string) error {
	listen, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	// создаём gRPC-сервер без зарегистрированной службы
	server.wrapped = grpc.NewServer(
		grpc.ChainUnaryInterceptor(server.trustedSubnetInterceptor, server.decryptionInterceptor),
	)
	// регистрируем сервис
	proto.RegisterLogoggerServer(server.wrapped, server)

	// получаем запрос gRPC
	return server.wrapped.Serve(listen)
}

func (server *GRPCServer) Shutdown(idleConnsClosed chan<- struct{}) error {
	server.wrapped.GracefulStop()
	close(idleConnsClosed)
	return nil
}

func (server *GRPCServer) decryptionInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	// is theoretically possible by using custom codecs, but I was
	// unable to do that in any sensible manner
	return handler(ctx, req)
}

func (server *GRPCServer) trustedSubnetInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	if server.trustedSubnet == nil {
		return handler(ctx, req)
	}

	var rawIP string
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		values := md.Get("X-Real-IP")
		if len(values) > 0 {
			rawIP = values[0]
		}
	}

	ip := net.ParseIP(rawIP)
	if ip == nil {
		return nil, status.Error(codes.Unauthenticated, "missing token")
	}

	if !server.trustedSubnet.Contains(ip) {
		return nil, status.Error(codes.Unauthenticated, "missing token")
	}
	return handler(ctx, req)
}

func (server *GRPCServer) WithDecryptor(decryptor crypt.Decryptor) Server {
	server.decryptor = decryptor
	return server
}

func (server *GRPCServer) WithTrustedSubnet(trustedSubnet *net.IPNet) Server {
	server.trustedSubnet = trustedSubnet
	return server
}

func NewGRPCServer(app *App) *GRPCServer {
	server := new(GRPCServer)
	server.app = app
	server.decryptor = crypt.NoOpDecryptor{}
	server.trustedSubnet = nil

	return server
}
