package mock_test

import (
	"testing"

	"github.com/revolyssup/k8sdebug/pkg/portforward/mock"
	"github.com/stretchr/testify/assert"
)

func TestRoundRobin(t *testing.T) {
	mock := mock.New("8080", "8081", "8082")

	assert.Equal(t, "8080", mock.NextPort())
	assert.Equal(t, "8081", mock.NextPort())
	assert.Equal(t, "8082", mock.NextPort())
	assert.Equal(t, "8080", mock.NextPort())
	assert.Equal(t, "8081", mock.NextPort())
	assert.Equal(t, "8082", mock.NextPort())
}
