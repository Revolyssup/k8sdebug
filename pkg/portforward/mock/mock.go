package mock

import "github.com/revolyssup/k8sdebug/pkg/forwarder"

type MockForwarder struct {
	ports []string
	index int
}

// NewMock creates a mock forwarder with predefined ports.
func New(ports ...string) forwarder.Forwarder {
	return &MockForwarder{
		ports: ports,
	}
}

// Port returns the next predefined port or an error if exhausted.
func (m *MockForwarder) NextPort() string {
	port := m.ports[m.index]
	m.index = (m.index + 1) % len(m.ports)
	return port
}
