package tests

import (
	"fmt"
	"net"
	"net/http"
	"sort"
	"sync"
	"testing"
	"time"
)

func BenchmarkL4LoadBalancerRoundRobin(b *testing.B) {
	address := "localhost:8082" // L4 로드 밸런서 주소 (IP:포트 형식)
	endpoint := "/l4/rr/api/v1/users"

	benchmarkL4LoadBalancer(b, address, endpoint)
}

func BenchmarkL4LoadBalancerLeastConnections(b *testing.B) {
	address := "localhost:8082" // L4 로드 밸런서 주소 (IP:포트 형식)
	endpoint := "/l4/lc/api/v1/users"

	benchmarkL4LoadBalancer(b, address, endpoint)
}

func BenchmarkL7LoadBalancerRoundRobin(b *testing.B) {
	url := "http://localhost:8082/l7/rr/api/v1/users" // L7 로드 밸런서 주소
	benchmarkL7LoadBalancer(b, url)
}

func BenchmarkL7LoadBalancerLeastConnections(b *testing.B) {
	url := "http://localhost:8082/l7/lc/api/v1/users" // L7 로드 밸런서 주소
	benchmarkL7LoadBalancer(b, url)
}

func benchmarkL7LoadBalancer(b *testing.B, url string) {
	var wg sync.WaitGroup
	var latencies []time.Duration
	var latenciesMutex sync.Mutex // latencies 접근 보호를 위한 Mutex

	maxRetries := 3 // 최대 재시도 횟수
	requestInterval := 30 * time.Millisecond

	for i := 0; i < b.N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			var latency time.Duration
			var success bool
			for attempt := 0; attempt < maxRetries; attempt++ {
				start := time.Now()
				resp, err := http.Get(url)
				if err == nil {
					latency = time.Since(start)
					resp.Body.Close()
					success = true
					break
				}

				// 재시도 로직
				b.Logf("Attempt %d failed with error: %v", attempt+1, err)
				time.Sleep(100 * time.Millisecond) // 재시도 전 대기
			}

			// 재시도가 모두 실패한 경우 로그 기록
			if !success {
				b.Errorf("Failed to send request to L7 load balancer after %d attempts", maxRetries)
				return
			}

			// latencies에 동시 접근 방지
			latenciesMutex.Lock()
			latencies = append(latencies, latency)
			latenciesMutex.Unlock()
		}()

		time.Sleep(requestInterval) // 요청 간 대기 시간 추가
	}
	wg.Wait()

	// TPS 및 95% 응답 속도 계산
	calculatePerformance(b, latencies)
}

func benchmarkL4LoadBalancer(b *testing.B, address string, endpoint string) {
	var wg sync.WaitGroup
	var latencies []time.Duration
	var latenciesMutex sync.Mutex // latencies 접근 보호를 위한 Mutex

	maxRetries := 3                          // 최대 재시도 횟수
	requestInterval := 10 * time.Millisecond // 요청 간 대기 시간

	for i := 0; i < b.N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			var latency time.Duration
			var success bool
			for attempt := 0; attempt < maxRetries; attempt++ {
				start := time.Now()
				conn, err := net.Dial("tcp", address)
				if err == nil {
					defer conn.Close()

					// HTTP 요청 형식으로 메시지 전송
					request := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n", endpoint)
					_, err = conn.Write([]byte(request))
					if err == nil {
						// 응답 수신 시간 측정
						buffer := make([]byte, 1024)
						conn.Read(buffer)
						latency = time.Since(start)
						success = true
						break
					}
				}

				// 재시도 로직
				b.Logf("Attempt %d failed with error: %v", attempt+1, err)
				time.Sleep(100 * time.Millisecond) // 재시도 전 대기
			}

			// 재시도가 모두 실패한 경우 로그 기록
			if !success {
				b.Errorf("Failed to send request to L4 load balancer after %d attempts", maxRetries)
				return
			}

			// latencies에 동시 접근 방지
			latenciesMutex.Lock()
			latencies = append(latencies, latency)
			latenciesMutex.Unlock()
		}()

		time.Sleep(requestInterval) // 요청 간 대기 시간 추가
	}
	wg.Wait()

	// TPS 및 95% 응답 속도 계산
	calculatePerformance(b, latencies)
}

// TPS 및 95% 응답 속도 계산 함수
func calculatePerformance(b *testing.B, latencies []time.Duration) {
	totalTime := time.Duration(0)
	for _, latency := range latencies {
		totalTime += latency
	}

	// TPS 계산
	avgTPS := float64(b.N) / totalTime.Seconds()
	b.Logf("TPS: %.2f", avgTPS)

	// 95% 응답 속도 계산
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})
	p95Latency := latencies[int(0.95*float64(len(latencies)))]
	b.Logf("95th percentile latency: %v", p95Latency)
}
