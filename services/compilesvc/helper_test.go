package compilesvc_test

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
)

func fakeServer(f http.HandlerFunc) (*http.Client, func()) {
	s := httptest.NewTLSServer(http.HandlerFunc(f))
	client := s.Client()
	client.Transport = &http.Transport{
		// #nosec G402
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return net.Dial(network, s.Listener.Addr().String())
		},
	}

	return client, s.Close
}
