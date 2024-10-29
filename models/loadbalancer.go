package models

import (
	"net/http/httputil"
	"net/url"
	"sync"
)

type L4Backend struct {
	URL         string
	Alive       bool
	Connections int
	Semaphore   chan struct{}
}

type L7Backend struct {
	URL          *url.URL
	Alive        bool
	ReverseProxy *httputil.ReverseProxy
	Connections  int
	Semaphore    chan struct{}
}

type L4LoadBalancer struct {
	Backends []*L4Backend
	Mutex    sync.RWMutex
}

type L7LoadBalancer struct {
	Backends []*L7Backend
	Mutex    sync.RWMutex
}
