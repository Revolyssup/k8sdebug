package sticky

import (
	"net"
	"sync"

	"github.com/revolyssup/k8sdebug/pkg/forwarder"
	"github.com/revolyssup/k8sdebug/pkg/portforward/roundrobin"
)

type StickySession struct {
	ipTracker  map[string]string // Source IP to Allotted port mapping
	mx         sync.Mutex
	connPool   *[]string
	roundRobin forwarder.Forwarder
}

func New(connPool *[]string) forwarder.Forwarder {
	return &StickySession{
		connPool:   connPool,
		ipTracker:  make(map[string]string),
		roundRobin: roundrobin.New(connPool),
	}
}
func (ss *StickySession) NextPort(conn net.Conn) string {
	ss.mx.Lock()
	defer ss.mx.Unlock()

	remoteAddr := conn.RemoteAddr().String()

	// Split into IP and port
	host, _, _ := net.SplitHostPort(remoteAddr)
	if ss.ipTracker[host] != "" {
		return ss.ipTracker[host]
	}
	//fallback to roundrobin
	port := ss.roundRobin.NextPort(conn)
	ss.ipTracker[host] = port
	return port
}
