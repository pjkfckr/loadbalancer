package main

import (
	"github.com/gin-gonic/gin"
	"loadbalancer-go/loadbalancer"
)

func main() {
	r := gin.Default()

	l4lb := loadbalancer.NewL4LoadBalancer([]string{"http://localhost:9993"}, 100)
	l7lb := loadbalancer.NewL7LoadBalancer([]string{"http://localhost:9998"}, 100)

	r.Any("/l4/rr/*proxyPath", l4lb.RoundRobinHandler)
	r.Any("/l4/lc/*proxyPath", l4lb.LeastConnectionHandler)
	r.Any("/l7/rr/*proxyPath", l7lb.RoundRobinHandler)
	r.Any("/l7/lc/*proxyPath", l7lb.LeastConnectionHandler)

	r.Run(":8082")
}
