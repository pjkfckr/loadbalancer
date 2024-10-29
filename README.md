
# L7 과 L4 로드밸런서 간의 벤치마크 테스트 진행

### 테스트 조건
- maxConnections = 100
- API 서버 (FastAPI) 0.1 ~ 0.3 Random Sleep 
  - uvicorn 으로 worker 는 2개로 진행
- Counting Semaphore 를 사용하여 동시성 제어
- L7 에는 IPHash 만 추가되어 다른 로직들이 추가되어있지않아 L4 와 비슷할 수 있습니다.


# L7 LoadBalancer

### RoundRobin with Counting Semaphore

```text
--- BENCH: BenchmarkL7LoadBalancerRoundRobin-11
    performance_test.go:155: TPS: 3.89
    performance_test.go:162: 95th percentile latency: 257.379709ms
    performance_test.go:155: TPS: 4.86
    performance_test.go:162: 95th percentile latency: 256.202375ms
    performance_test.go:155: TPS: 4.51
    performance_test.go:162: 95th percentile latency: 297.555875ms
    performance_test.go:155: TPS: 4.77
    performance_test.go:162: 95th percentile latency: 286.024959ms
    performance_test.go:155: TPS: 4.75
    performance_test.go:162: 95th percentile latency: 291.202458ms
	... [output truncated]
```


### LeastConnections with Counting Semaphore
```text
--- BENCH: BenchmarkL7LoadBalancerLeastConnections-11
    performance_test.go:155: TPS: 4.25
    performance_test.go:162: 95th percentile latency: 235.488791ms
    performance_test.go:155: TPS: 4.80
    performance_test.go:162: 95th percentile latency: 266.925875ms
    performance_test.go:155: TPS: 4.76
    performance_test.go:162: 95th percentile latency: 300.71325ms
    performance_test.go:155: TPS: 4.62
    performance_test.go:162: 95th percentile latency: 286.970625ms
    performance_test.go:155: TPS: 5.06
    performance_test.go:162: 95th percentile latency: 295.801292ms
	... [output truncated]
```


# L4 LoadBalancer

### RoundRobin With Counting Semaphore
```text
--- BENCH: BenchmarkL4LoadBalancerRoundRobin-11
    performance_test.go:155: TPS: 3.30
    performance_test.go:162: 95th percentile latency: 303.02575ms
    performance_test.go:155: TPS: 4.99
    performance_test.go:162: 95th percentile latency: 247.354958ms
    performance_test.go:155: TPS: 4.94
    performance_test.go:162: 95th percentile latency: 312.584792ms
    performance_test.go:155: TPS: 4.69
    performance_test.go:162: 95th percentile latency: 292.889541ms
    performance_test.go:155: TPS: 4.77
    performance_test.go:162: 95th percentile latency: 297.330125ms
	... [output truncated]
```

### LeastConnections with Counting Semaphore
```text
--- BENCH: BenchmarkL4LoadBalancerLeastConnections-11
    performance_test.go:155: TPS: 3.76
    performance_test.go:162: 95th percentile latency: 265.84375ms
    performance_test.go:155: TPS: 5.00
    performance_test.go:162: 95th percentile latency: 236.96025ms
    performance_test.go:155: TPS: 5.74
    performance_test.go:162: 95th percentile latency: 240.02525ms
    performance_test.go:155: TPS: 4.76
    performance_test.go:162: 95th percentile latency: 296.965708ms
    performance_test.go:155: TPS: 4.88
    performance_test.go:162: 95th percentile latency: 298.201666ms
	... [output truncated]
```


### 벤치마크 요약
1. L4 Round Robin

   •	평균 TPS: 약 4.0 ~ 5.5
   •	95% 지연 시간: 235ms ~ 310ms

2. L4 Least Connections

   •	평균 TPS: 약 4.0 ~ 5.5
   •	95% 지연 시간: 230ms ~ 300ms

3. L7 Round Robin

   •	평균 TPS: 약 3.5 ~ 8.0
   •	95% 지연 시간: 120ms ~ 300ms

4. L7 Least Connections

   •	평균 TPS: 약 4.0 ~ 7.0
   •	95% 지연 시간: 140ms ~ 300ms

### L7 로드밸런서에서 추가할 수 있는 기능 및 로직
1. 헤더 및 쿠키 기반 세션 유지 (Session Persistence)

   •	특정 사용자나 세션을 같은 백엔드 서버로 유지하는 로직을 추가할 수 있습니다. 이를 통해 세션 일관성을 유지하고, 상태 유지가 필요한 애플리케이션에서 성능이 향상됩니다.

2. 경로 기반 라우팅 (Path-based Routing)

   •	URL 경로를 기반으로 요청을 분산할 수 있습니다. 예를 들어, /api 요청은 API 서버로, /static 요청은 CDN이나 파일 서버로 라우팅하여 리소스 요청 분산이 가능해져 효율성을 높일 수 있습니다.

3. 요청 크기 기반 라우팅

   •	요청의 Content-Length나 데이터 유형을 검사하여 작은 요청은 빠르게 처리할 수 있는 서버로, 큰 요청은 성능이 높은 서버로 분산할 수 있습니다. 이를 통해 리소스 사용을 최적화할 수 있습니다.

4. 상태 점검 및 동적 백엔드 관리 (Health Checks)

   •	백엔드의 상태를 주기적으로 점검하여 응답 시간이 느리거나 장애가 발생한 서버를 자동으로 제외하거나, 응답이 좋은 서버에 더 많은 요청을 할당하는 방식으로 트래픽을 동적으로 조정할 수 있습니다.

5. 캐싱

   •	응답 데이터를 캐싱해 중복된 요청이 발생할 경우 캐시에서 응답을 제공하면 백엔드 서버 부하를 줄일 수 있습니다. 특히 정적인 리소스에 대해 유용하며, 캐싱 정책에 따라 효율적인 성능 향상이 가능합니다.

6. TLS 종료 및 보안 처리

   •	L7 로드 밸런서는 HTTPS 트래픽을 해석할 수 있기 때문에, SSL/TLS 인증 및 종료 작업을 수행하여 백엔드 서버의 암호화 부담을 덜어주는 역할을 할 수 있습니다. 또한 특정 요청에 대해 보안 정책을 설정하는 것도 가능합니다.

7. QoS (Quality of Service)

   •	중요한 요청에 우선순위를 두고 처리하는 로직을 추가할 수 있습니다. 예를 들어, 사용자 인증 요청은 더 빨리 처리하고, 덜 중요한 배경 작업 요청은 나중에 처리하는 방식입니다.

   •	L4 로드 밸런서는 단순히 IP와 포트 기반으로 트래픽을 분산하므로 오버헤드가 적고 TPS가 높으며, 지연 시간이 짧은 경향이 있습니다. 이를 통해 단순한 연결 기반 분산 처리에서는 L4가 더 효율적입니다.
   •	L7 로드 밸런서는 헤더와 경로 기반 라우팅 등의 추가적인 프로세싱이 가능하지만, 오버헤드로 인해 TPS가 낮고 지연 시간이 길어질 수 있습니다. 그러나 L7은 HTTP 계층에서 트래픽을 세밀하게 제어하고 특정 트래픽 유형을 식별하여 효율적으로 분산할 수 있어 복잡한 분산 요구가 있는 환경에서 더 적합합니다.


### L4 vs L7 로드 밸런서

- L4 로드 밸런서는 단순히 IP와 포트 기반으로 트래픽을 분산하므로 오버헤드가 적고 TPS가 높으며, 지연 시간이 짧은 경향이 있습니다. 이를 통해 단순한 연결 기반 분산 처리에서는 L4가 더 효율적입니다. 
- 로드 밸런서는 헤더와 경로 기반 라우팅 등의 추가적인 프로세싱이 가능하지만, 오버헤드로 인해 TPS가 낮고 지연 시간이 길어질 수 있습니다. 그러나 L7은 HTTP 계층에서 트래픽을 세밀하게 제어하고 특정 트래픽 유형을 식별하여 효율적으로 분산할 수 있어 복잡한 분산 요구가 있는 환경에서 더 적합합니다.

### Round Robin vs Least Connections

- Round Robin 방식은 요청을 순서대로 분산하므로, 짧은 요청과 긴 요청이 혼재된 환경에서는 로드의 편차가 발생할 가능성이 있습니다. 이로 인해 특정 백엔드 서버에 더 큰 부하가 집중될 수 있으며, 지연 시간의 변동폭이 커질 수 있습니다. 
- Least Connections 방식은 연결 수가 적은 서버로 요청을 분산하기 때문에, 상대적으로 부하가 고르게 분산되어 지연 시간이 더 일정한 경향이 있습니다. 이로 인해 L4와 L7 모두에서 Least Connections 방식의 TPS가 약간 더 높고, 지연 시간이 일정하게 나타날 수 있습니다.