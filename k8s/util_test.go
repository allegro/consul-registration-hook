package k8s

import (
	"testing"
	"time"

	corev1 "github.com/ericchiang/k8s/apis/core/v1"
	"github.com/ericchiang/k8s/util/intstr"
	"github.com/stretchr/testify/assert"

	"github.com/wix-playground/consul-registration-hook/consul"
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
		Handler: &corev1.Handler{
			HttpGet: &corev1.HTTPGetAction{
				Host: &localhost,
				Path: &path,
				Port: &intstr.IntOrString{IntVal: &port},
			},
		},
		PeriodSeconds:  &sixtySeconds,
		TimeoutSeconds: &sixtySeconds,
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
		Handler: &corev1.Handler{
			TcpSocket: &corev1.TCPSocketAction{
				Port: &intstr.IntOrString{IntVal: &port},
			},
		},
		PeriodSeconds:  &sixtySeconds,
		TimeoutSeconds: &sixtySeconds,
	}

	httpCheck := ConvertToConsulCheck(httpProbe, "localhost")

	assert.Equal(t, consul.CheckTCP, httpCheck.Type)
	assert.Equal(t, "localhost:8080", httpCheck.Address)
	assert.Equal(t, time.Minute, httpCheck.Timeout)
	assert.Equal(t, time.Minute, httpCheck.Interval)
}
