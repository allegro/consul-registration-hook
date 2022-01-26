package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
)

var defaultSCID = "10698"
var envSCID = "HOOK_SCID"
var envLoggerAddress = "HOOK_LOGGER_ADDRESS"
var defaultLoggerAddress = "endpoint.logger-dev.qxlint:515"

type Msg struct {
	SCID      string `json:"scid"`
	Message   string `json:"message"`
	Namespace string `json:"namespace"`
	Pod       string `json:"pod"`
	Variant   string `json:"variant"`
}

type KibanaWriter struct {
	pod           string
	namespace     string
	variant       string
	scid          string
	netWrite      net.Conn
	loggerAddress string
}

func NewKibanaWriter() (*KibanaWriter, error) {
	scid, loggerAddress := "", ""
	if scid = os.Getenv(envSCID); scid == "" {
		scid = defaultSCID
	}
	if loggerAddress = os.Getenv(envLoggerAddress); loggerAddress == "" {
		loggerAddress = defaultLoggerAddress
	}

	pod := os.Getenv("KUBERNETES_POD_NAME")
	namespace := os.Getenv("KUBERNETES_POD_NAMESPACE")
	variant := os.Getenv("VARIANT_NAME")
	return &KibanaWriter{
		loggerAddress: loggerAddress,
		pod:           pod,
		namespace:     namespace,
		variant:       variant,
		scid:          scid,
	}, nil
}

func (k KibanaWriter) Write(p []byte) (n int, err error) {
	msg := &Msg{
		Message:   string(p),
		SCID:      k.scid,
		Pod:       k.pod,
		Namespace: k.namespace,
		Variant:   k.variant,
	}
	netWrite, err := net.Dial("tcp", k.loggerAddress)
	if err != nil {
		return 0, fmt.Errorf("error: %s", err)
	}
	defer netWrite.Close()

	writeTemplate, err := json.Marshal(msg)
	if err != nil {
		fmt.Println(err)
		return
	}
	return netWrite.Write(writeTemplate)
}

func ConfigureLogger() {
	kibanaWriter, err := NewKibanaWriter()
	if err != nil {
		log.Printf("Unable to initialize kibana logger: %s", err)
	}
	log.SetOutput(io.MultiWriter(os.Stdout, kibanaWriter))
}
