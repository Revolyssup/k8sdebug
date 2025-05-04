package roundrobin

import (
	"net"
	"sync"
	"time"

	"github.com/revolyssup/k8sdebug/pkg/forwarder"
)

type RoundRobin struct {
	connNumber int
	mx         sync.Mutex
	connPool   *[]string
}

func New(connPool *[]string) forwarder.Forwarder {
	return &RoundRobin{
		connPool: connPool,
	}
}
func (rr *RoundRobin) NextPort() string {
	rr.mx.Lock()
	defer rr.mx.Unlock()

	initial := rr.connNumber
	if rr.connPool == nil {
		return ""
	}
	if len(*rr.connPool) == 0 {
		return ""
	}
	for {
		rr.connNumber = (rr.connNumber + 1) % len(*rr.connPool)
		portNum := (*rr.connPool)[rr.connNumber]
		if portNum != "" {
			// Check if port is actually listening
			conn, err := net.DialTimeout("tcp", ":"+portNum, 50*time.Millisecond)
			if err == nil {
				conn.Close()
				return portNum
			}
		}
		if rr.connNumber == initial {
			break // Avoid infinite loop
		}
	}
	return ""
}
