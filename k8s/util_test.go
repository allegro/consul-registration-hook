package k8s

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestHttpEndpointCHeck(t *testing.T) {

	ip := "127.0.0.1"
	path := "/status/ping"
	testServer := httptest.NewServer(http.HandlerFunc(
		func(res http.ResponseWriter, req *http.Request) {
			switch req.RequestURI {
			case path:
				res.Header().Set("Content-Type", "text/html")
			default:
				http.Error(res, "", http.StatusNotFound)
				return
			}
		},
	),
	)
	defer testServer.Close()
	_, port, _ := net.SplitHostPort(testServer.Listener.Addr().String())
	url := fmt.Sprintf("http://%s:%s%s", ip, port, path)

	assert.NoError(t, doHTTPCheck(url))

	port = "1000"
	url = fmt.Sprintf("http://%s:%s%s", ip, port, path)
	assert.Error(t, doHTTPCheck(url))
}

func TestTCPEndpointCHeck(t *testing.T) {

	ip := "127.0.0.1"
	port := "40000"
	l, err := net.Listen("tcp", ip+":"+port)
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}
	defer l.Close()

	assert.NoError(t, doTCPCheck(ip, port))

	port = "1000"
	assert.Error(t, doTCPCheck(ip, port))
}
