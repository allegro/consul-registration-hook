package consul

import (
	"testing"
	"time"

	"github.com/hashicorp/consul/testutil"
	"github.com/stretchr/testify/require"
)

func TestIfRegistersAndDeregistersServiceInConsul(t *testing.T) {
	srv, err := testutil.NewTestServerConfig(func(c *testutil.TestServerConfig) {
		c.Ports = &testutil.TestPortConfig{HTTP: 8500}
	})
	require.NoError(t, err)
	defer srv.Stop()

	service := ServiceInstance{
		ID:   "id",
		Name: "serviceName",
		Check: &Check{
			Type:     CheckTCP,
			Address:  "localhost:8500",
			Interval: time.Second,
			Timeout:  time.Second,
		},
	}

	err = Register([]ServiceInstance{service})
	require.NoError(t, err)

	err = Deregister([]ServiceInstance{service})
	require.NoError(t, err)
}
