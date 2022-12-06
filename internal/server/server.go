package server

import (
	"logogger/internal/crypt"
	"net"
)

type Server interface {
	Serve(address string) error
	Shutdown(chan<- struct{}) error
	WithDecryptor(decryptor crypt.Decryptor) Server
	WithTrustedSubnet(trustedSubnet *net.IPNet) Server
}
