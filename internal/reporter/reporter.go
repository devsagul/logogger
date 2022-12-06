package reporter

import (
	"context"
	"errors"
	"net"

	"logogger/internal/schema"
)

type Reporter interface {
	ReportMetricsBatches(ctx context.Context, l []schema.Metrics, host string) error
	Shutdown()
}

func getRealIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			return ipnet.IP.String(), nil
		}
	}
	return "", errors.New("unable to obtain IP address")
}
