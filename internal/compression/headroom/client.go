package headroom

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
)

type Client struct {
	endpoint   *url.URL
	httpClient *http.Client
}

func NewClient(endpoint string, httpClient *http.Client) (*Client, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse headroom endpoint: %w", err)
	}
	if u.Scheme != "http" {
		return nil, errors.New("headroom endpoint must use http")
	}
	if u.User != nil {
		return nil, errors.New("headroom endpoint must not contain user information")
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("headroom endpoint must not contain a query or fragment")
	}
	if u.Path != "/v1/compress" {
		return nil, errors.New("headroom endpoint path must be /v1/compress")
	}
	if !isLoopbackHost(u.Hostname()) {
		return nil, errors.New("headroom endpoint must use a loopback address")
	}
	if u.Port() == "" {
		return nil, errors.New("headroom endpoint must include a port")
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	clientCopy := *httpClient
	clientCopy.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &Client{endpoint: u, httpClient: &clientCopy}, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
