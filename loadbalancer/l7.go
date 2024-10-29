package loadbalancer

import (
	"github.com/gin-gonic/gin"
	"hash/fnv"
	"loadbalancer-go/models"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
)

type L7LoadBalancer struct {
	*models.L7LoadBalancer
}

// 해시 함수: IP 주소를 해시로 변환
func hashIP(ip string) int {
	h := fnv.New32a()
	h.Write([]byte(ip))
	return int(h.Sum32())
}

// 백엔드 선택 함수: IP 해시 기반으로 백엔드 선택
func (lb *L7LoadBalancer) selectBackend(ip string) *models.L7Backend {
	lb.Mutex.RLock()
	defer lb.Mutex.RUnlock()

	// 백엔드가 0개일 경우 처리
	if len(lb.Backends) == 0 {
		return nil
	}

	// IP 해시 값을 이용하여 백엔드 선택
	hash := hashIP(ip)
	backendIndex := hash % len(lb.Backends)
	return lb.Backends[backendIndex]
}

// 새로운 백엔드 초기화 시 Counting Semaphore 설정
func NewL7Backend(url *url.URL, maxConnections int) *models.L7Backend {
	return &models.L7Backend{
		URL:          url,
		Alive:        true,
		ReverseProxy: httputil.NewSingleHostReverseProxy(url),
		Semaphore:    make(chan struct{}, maxConnections), // Counting Semaphore 초기화
	}
}

func NewL7LoadBalancer(backends []string, maxConnections int) *L7LoadBalancer {
	lb := &models.L7LoadBalancer{
		Backends: make([]*models.L7Backend, len(backends)),
		Mutex:    sync.RWMutex{},
	}

	for i, b := range backends {
		// string 타입을 *url.URL 타입으로 변환
		parsedURL, err := url.Parse(b)
		if err != nil {
			// URL 파싱 에러 처리
			continue // 혹은 에러를 로깅하고 다음으로 넘어감
		}
		lb.Backends[i] = NewL7Backend(parsedURL, maxConnections)
	}
	return &L7LoadBalancer{L7LoadBalancer: lb}
}

func (lb *L7LoadBalancer) IPHashHandler(c *gin.Context) {
	lb.Mutex.RLock()
	defer lb.Mutex.RUnlock()

	clientIP := c.ClientIP()
	backend := lb.selectBackend(clientIP)

	// 유효한 백엔드가 없을 경우 오류 반환
	if backend == nil || !backend.Alive {
		c.JSON(503, gin.H{"error": "Service Unavailable"})
		return
	}

	// IP 해시에 따라 선택된 백엔드에 요청 전달
	backend.Connections++                               // 연결 수 증가
	backend.ReverseProxy.ServeHTTP(c.Writer, c.Request) // 요청 전달
	backend.Connections--                               // 연결 완료 후 감소
}

func (lb *L7LoadBalancer) RoundRobinHandler(c *gin.Context) {
	lb.Mutex.Lock()
	// Round-robin 방식으로 백엔드를 선택
	backend := lb.Backends[0]
	lb.Backends = append(lb.Backends[1:], backend)
	lb.Mutex.Unlock()

	// 세마포어가 가득 찬 경우 대기
	if backend == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "No available backend"})
		return
	}

	// 세마포어에 접근할 수 있을 때까지 대기
	backend.Semaphore <- struct{}{}
	defer func() { <-backend.Semaphore }() // 요청 처리 후 세마포어 해제

	// 백엔드의 현재 연결 수 증가
	lb.Mutex.Lock()
	backend.Connections++
	lb.Mutex.Unlock()

	defer func() {
		lb.Mutex.Lock()
		backend.Connections--
		lb.Mutex.Unlock()
	}()

	originalPath := c.Param("proxyPath")
	c.Request.URL.Path = originalPath

	// 헤더 기반 라우팅 로직 예시
	if c.GetHeader("X-Custom-Header") == "special" {
		// 특정 백엔드로 라우팅 로직 추가
	}

	// 요청을 백엔드로 전달
	backend.ReverseProxy.ServeHTTP(c.Writer, c.Request)
}

func (lb *L7LoadBalancer) LeastConnectionHandler(c *gin.Context) {
	lb.Mutex.Lock()
	var minConn *models.L7Backend
	for _, b := range lb.Backends {
		if minConn == nil || b.Connections < minConn.Connections {
			minConn = b
		}
	}
	lb.Mutex.Unlock()

	// 세마포어가 처리할 수 있을때까지 대기
	if minConn == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "No available backend"})
		return
	}

	minConn.Semaphore <- struct{}{}
	defer func() { <-minConn.Semaphore }() // 요청 처리 후 세마포어 해제

	//백엔드의 현재 연결 수 증가
	lb.Mutex.Lock()
	minConn.Connections++
	lb.Mutex.Unlock()

	defer func() {
		lb.Mutex.Lock()
		minConn.Connections--
		lb.Mutex.Unlock()
	}()

	originalPath := c.Param("proxyPath")
	c.Request.URL.Path = originalPath

	// 경로 기반 라우팅 로직 예시
	if c.Request.URL.Path == "/api" {
		// 특정 백엔드로 라우팅 로직 추가
	}

	// 요청을 백엔드로 전달
	minConn.ReverseProxy.ServeHTTP(c.Writer, c.Request)
}
