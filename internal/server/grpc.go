package server

import "logogger/internal/proto"

type LogoggerServer struct {
	proto.UnimplementedLogoggerServer

	// the application
}
