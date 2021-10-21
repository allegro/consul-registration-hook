package k8s

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/allegro/consul-registration-hook/consul"
	corev1 "k8s.io/api/core/v1"
)

const (
	defaultScheme = "http"
	connTimeOut   = 2
)

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

func doHTTPCheck(url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(connTimeOut*time.Second))
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	if _, err := http.DefaultClient.Do(request); err != nil {
		return err
	}

	return nil
}

func doTCPCheck(ip, port string) error {
	timeout := time.Duration(connTimeOut) * time.Second
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%s", ip, port), timeout)
	if err != nil {
		return err
	}
	if err = conn.Close(); err != nil {
		return err
	}
	return nil
}
