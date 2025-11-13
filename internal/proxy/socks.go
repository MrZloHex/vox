package proxy

import (
	"context"
	"golang.org/x/net/proxy"
	"net"
	"net/http"
	"time"
)

func NewSocksClient(socksAddr string) (*http.Client, error) {
	dialer, err := proxy.SOCKS5("tcp", socksAddr, nil, proxy.Direct)
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
	}

	return &http.Client{
		Transport: transport,
		Timeout:   120 * time.Second,
	}, nil
}
