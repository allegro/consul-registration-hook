package k8s

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"

	"github.com/allegro/consul-registration-hook/consul"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestIfConvertsNilProbeToNilCheck(t *testing.T) {
	assert.Nil(t, ConvertToConsulCheck(nil, ""))
}

func TestIfConvertsHTTPProbeToHTTPCheck(t *testing.T) {
	sixtySeconds := int32(60)
	localhost := "localhost"
	path := "/ping"
	port := int32(8080)

	httpProbe := &corev1.Probe{
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Host: localhost,
				Path: path,
				Port: intstr.IntOrString{IntVal: port},
			},
		},
		PeriodSeconds:  sixtySeconds,
		TimeoutSeconds: sixtySeconds,
	}

	httpCheck := ConvertToConsulCheck(httpProbe, "localhost")

	assert.Equal(t, consul.CheckHTTPGet, httpCheck.Type)
	assert.Equal(t, "http://localhost:8080/ping", httpCheck.Address)
	assert.Equal(t, time.Minute, httpCheck.Timeout)
	assert.Equal(t, time.Minute, httpCheck.Interval)
}

func TestIfConvertsTCPProbeToTCPCheck(t *testing.T) {
	sixtySeconds := int32(60)
	port := int32(8080)

	httpProbe := &corev1.Probe{
		Handler: corev1.Handler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.IntOrString{IntVal: port},
			},
		},
		PeriodSeconds:  sixtySeconds,
		TimeoutSeconds: sixtySeconds,
	}

	httpCheck := ConvertToConsulCheck(httpProbe, "localhost")

	assert.Equal(t, consul.CheckTCP, httpCheck.Type)
	assert.Equal(t, "localhost:8080", httpCheck.Address)
	assert.Equal(t, time.Minute, httpCheck.Timeout)
	assert.Equal(t, time.Minute, httpCheck.Interval)
}
