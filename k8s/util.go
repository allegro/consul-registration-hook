package k8s

import (
	"net"
	"net/url"
	"strconv"
	"time"

	// corev1 "github.com/ericchiang/k8s/apis/core/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/allegro/consul-registration-hook/consul"
)

const defaultScheme = "http"

// ConvertToConsulCheck converts Kubernetes probe definition to Consul check
// definition.
func ConvertToConsulCheck(probe *corev1.Probe, host string) *consul.Check {
	if probe == nil {
		return nil
	}

	var checkType consul.CheckType
	var address string

	if handler := probe.Handler.HTTPGet; handler != nil {
		checkType = consul.CheckHTTPGet
		u := url.URL{
			Host:   net.JoinHostPort(host, strconv.Itoa(int(handler.Port.IntVal))),
			Path:   handler.Path,
			Scheme: defaultScheme, // Consul do not support HTTPS checks
		}
		address = u.String()
	} else if handler := probe.Handler.TCPSocket; handler != nil {
		checkType = consul.CheckTCP
		address = net.JoinHostPort(host, strconv.Itoa(int(handler.Port.IntVal)))
	} else {
		return nil
	}

	interval := time.Duration(probe.PeriodSeconds) * time.Second
	timeout := time.Duration(probe.TimeoutSeconds) * time.Second

	return &consul.Check{
		Type:     checkType,
		Address:  address,
		Interval: interval,
		Timeout:  timeout,
	}
}
