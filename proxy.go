package hraftd

import (
	"context"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"
)

// XOriginRemoteAddr is the const of header for original remote addr to real address sourcing.
const XOriginRemoteAddr = "X-Origin-RemoteAddr"

// ReverseProxy reverse proxy originalPath to targetHost with targetPath.
func ReverseProxy(targetHost, targetPath string, timeout time.Duration) *httputil.ReverseProxy {
	director := func(req *http.Request) {
		req.URL.Scheme = "http"

		req.URL.Host = targetHost
		req.URL.Path = targetPath

		req.Header.Set(XOriginRemoteAddr, req.RemoteAddr)
		req.Header.Add("X-Forwarded-Host", req.Host)
		req.Header.Add("X-Origin-Host", req.Header.Get("Host"))
	}

	t := &http.Transport{DialContext: TimeoutDialer(timeout, timeout)}

	// 更多可以参见 https://github.com/Integralist/go-reverse-proxy/blob/master/proxy/proxy.go
	return &httputil.ReverseProxy{Director: director, Transport: t}
}

// IsRelativeForward tells the statusCode is 301/302 and locationHeader is relative
func IsRelativeForward(statusCode int, locationHeader string) bool {
	switch statusCode {
	case http.StatusMovedPermanently, http.StatusFound:
	default:
		return false
	}

	return !HasPrefix(locationHeader, "http://", "https://")
}

// HasPrefix tells s has any prefix of p...
func HasPrefix(s string, p ...string) bool {
	for _, i := range p {
		if strings.HasPrefix(s, i) {
			return true
		}
	}

	return false
}

// Dialer defines dialer function alias
type Dialer func(ctx context.Context, net, addr string) (c net.Conn, err error)

// TimeoutDialer returns functions of connection dialer with timeout settings for http.Transport Dial field.
// https://gist.github.com/c4milo/275abc6eccbfd88ad56ca7c77947883a
// HTTP client with support for read and write timeouts which are missing in Go's standard library.
func TimeoutDialer(cTimeout time.Duration, rwTimeout time.Duration) Dialer {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := net.DialTimeout(network, addr, cTimeout)
		if err != nil {
			return conn, err
		}

		if rwTimeout > 0 {
			err = conn.SetDeadline(time.Now().Add(rwTimeout))
		}

		return conn, err
	}
}
