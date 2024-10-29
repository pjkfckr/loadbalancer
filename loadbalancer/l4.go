package loadbalancer

import (
	"github.com/gin-gonic/gin"
	"io"
	"loadbalancer-go/models"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
)

type L4LoadBalancer struct {
	*models.L4LoadBalancer
	currentBackend int
	mu             sync.Mutex
}

func NewL4Backend(url string, maxConnections int) *models.L4Backend {
	return &models.L4Backend{
		URL:       url,
		Alive:     true,
		Semaphore: make(chan struct{}, maxConnections), // 최대 동시 연결 수 지정
	}
}

func NewL4LoadBalancer(backends []string, maxConnections int) *L4LoadBalancer {
	lb := &models.L4LoadBalancer{
		Backends: make([]*models.L4Backend, len(backends)),
		Mutex:    sync.RWMutex{},
	}
	for i, b := range backends {
		lb.Backends[i] = NewL4Backend(b, maxConnections)
	}
	return &L4LoadBalancer{L4LoadBalancer: lb}
}

func (lb *L4LoadBalancer) Start(address string) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go lb.handleConnection(conn)
	}
}

func (lb *L4LoadBalancer) handleConnection(clientConn net.Conn) {
	defer clientConn.Close()

	backend := lb.getNextBackend()
	serverConn, err := net.Dial("tcp", backend.URL)
	if err != nil {
		return
	}
	defer serverConn.Close()

	backend.Connections++
	defer func() { backend.Connections-- }()

	go io.Copy(serverConn, clientConn)
	io.Copy(clientConn, serverConn)
}

func (lb *L4LoadBalancer) getNextBackend() *models.L4Backend {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	backend := lb.Backends[lb.currentBackend]
	lb.currentBackend = (lb.currentBackend + 1) % len(lb.Backends)
	return backend
}

func (lb *L4LoadBalancer) l4LeastConnection() *models.L4Backend {
	lb.Mutex.RLock()
	defer lb.Mutex.RUnlock()

	var minConn *models.L4Backend
	for _, b := range lb.Backends {
		if minConn == nil || b.Connections < minConn.Connections {
			minConn = b
		}
	}
	return minConn
}

func (lb *L4LoadBalancer) RoundRobinHandler(c *gin.Context) {
	backend := lb.getNextBackend()

	// 세마포어가 가득 찬 경우 대기
	if backend == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "No available backend"})
		return
	}

	// 세마포어에 접근할 수 있을 때까지 대기
	backend.Semaphore <- struct{}{}
	defer func() { <-backend.Semaphore }() // 요청 처리 후 세마포어 해제

	// 백엔드 URL 파싱
	targetURL, err := url.Parse(backend.URL)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error parsing backend URL")
		return
	}

	// 프록시 생성
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// *proxyPath에서 실제 경로 추출
	path := c.Param("proxyPath")
	if path == "" {
		path = "/"
	}

	// 원본 요청 헤더 설정
	c.Request.URL.Path = path
	c.Request.URL.RawPath = path
	c.Request.URL.Host = targetURL.Host
	c.Request.URL.Scheme = targetURL.Scheme
	c.Request.Header.Set("X-Forwarded-Host", c.Request.Header.Get("Host"))

	// 백엔드의 현재 연결 수 증가
	backend.Connections++
	defer func() { backend.Connections-- }()

	// 프록시를 통해 요청 전달
	proxy.ServeHTTP(c.Writer, c.Request)
}

func (lb *L4LoadBalancer) LeastConnectionHandler(c *gin.Context) {
	backend := lb.l4LeastConnection()

	// 세마포어가 가득 찬 경우 대기
	if backend == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "No available backend"})
		return
	}

	// 세마포어에 접근할 수 있을 때까지 대기
	backend.Semaphore <- struct{}{}
	defer func() { <-backend.Semaphore }() // 요청 처리 후 세마포어 해제

	// 백엔드 URL 파싱
	targetURL, err := url.Parse(backend.URL)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error parsing backend URL")
		return
	}

	// 프록시 생성
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// *proxyPath에서 실제 경로 추출
	path := c.Param("proxyPath")
	if path == "" {
		path = "/"
	}

	// 원본 요청 URL 수정
	c.Request.URL.Path = path
	c.Request.URL.RawPath = path
	c.Request.URL.Host = targetURL.Host
	c.Request.URL.Scheme = targetURL.Scheme
	c.Request.Header.Set("X-Forwarded-Host", c.Request.Header.Get("Host"))

	// 백엔드의 현재 연결 수 증가
	backend.Connections++
	defer func() { backend.Connections-- }()

	// 프록시를 통해 요청 전달
	proxy.ServeHTTP(c.Writer, c.Request)
}
