package k8s

import (
	"net"
	"net/url"
	"strconv"
	"time"

	corev1 "github.com/ericchiang/k8s/apis/core/v1"

	"github.com/allegro/consul-registration-hook/consul"
)

const defaultScheme = "http"

// ConvertToConsulCheck converts Kubernetes probe definition to Consul check
// definition.
func ConvertToConsulCheck(probe *corev1.Probe) *consul.Check {
	if probe == nil {
		return nil
	}

	var checkType consul.CheckType
	var address string

	if handler := probe.Handler.HttpGet; handler != nil {
		checkType = consul.CheckHTTPGet
		u := url.URL{
			Host:   net.JoinHostPort(handler.GetHost(), strconv.Itoa(int(handler.GetPort().GetIntVal()))),
			Path:   handler.GetPath(),
			Scheme: defaultScheme, // Consul do not support HTTPS checks
		}
		address = u.String()
	} else if handler := probe.Handler.TcpSocket; handler != nil {
		checkType = consul.CheckTCP
		address = net.JoinHostPort(handler.GetHost(), strconv.Itoa(int(handler.GetPort().GetIntVal())))
	} else {
		return nil
	}

	interval := time.Duration(probe.GetPeriodSeconds()) * time.Second
	timeout := time.Duration(probe.GetTimeoutSeconds()) * time.Second

	return &consul.Check{
		Type:     checkType,
		Address:  address,
		Interval: interval,
		Timeout:  timeout,
	}
}
