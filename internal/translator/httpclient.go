package translator

import (
	"net/http"
	"time"
)

// newHTTPClient builds the HTTP client used by the translator providers.
//
// timeoutSec is the request timeout in seconds; 0 keeps the historical
// default of 30 seconds.
//
// proxy selects the network egress:
//
//	true  → the transport keeps http.DefaultTransport's Proxy func, i.e.
//	        http.ProxyFromEnvironment, so HTTP_PROXY / HTTPS_PROXY /
//	        NO_PROXY are honoured. With no environment proxy set, requests
//	        go direct — true does not mean "force traffic through a proxy".
//	false → the transport's Proxy func is cleared, so the standard library
//	        dials directly and never consults HTTP_PROXY / HTTPS_PROXY.
//
// The transport is a Clone of http.DefaultTransport: the globals
// http.DefaultTransport and http.DefaultClient are never modified, and no
// dial/pool/HTTP2 tuning is re-invented here. Loopback destinations stay
// direct regardless of this flag, which is standard library behaviour
// (NO_PROXY semantics for localhost/loopback).
func newHTTPClient(timeoutSec int, proxy bool) *http.Client {
	timeout := time.Duration(timeoutSec) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if !proxy {
		transport.Proxy = nil
	}
	return &http.Client{Timeout: timeout, Transport: transport}
}
