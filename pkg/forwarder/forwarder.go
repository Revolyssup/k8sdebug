package forwarder

import (
	"net"
)

type Forwarder interface {
	NextPort(net.Conn) string
}
