package k8s

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
)

const (
	portDefinitionsEnv = "PORT_DEFINITIONS"
	probeLabel         = "probe"
	serviceLabel       = "service"
	consulLabel        = "consul"
)

type portDefinitions []portDefinition

type portDefinition struct {
	Port   int   `json:"port"`
	Labels label `json:"labels"`
}

type label map[string]string

func (pd portDefinition) getTags() []string {
	var tags []string
	for key, value := range pd.Labels {
		if value == "tag" {
			tags = append(tags, key)
		}
	}
	return tags
}

func (pd portDefinition) isService() bool {
	return pd.hasServiceLabel()
}

func (pd portDefinition) isProbe() bool {
	return pd.hasProbeLabel()
}

func (pd portDefinition) labelForConsul() string {
	if pd.hasConsulLabel() {
		return pd.Labels[consulLabel]
	}
	return ""
}

func (pd portDefinition) hasConsulLabel() bool {
	if _, ok := pd.Labels[consulLabel]; ok {
		return true
	}
	return false
}

func (pd portDefinition) hasServiceLabel() bool {
	if val, ok := pd.Labels[serviceLabel]; ok && val == "true" {
		return true
	}
	return false
}

func (pd portDefinition) hasProbeLabel() bool {
	if val, ok := pd.Labels[probeLabel]; ok && val == "true" {
		return true
	}
	return false
}

func getPortDefinitions() (*portDefinitions, error) {
	portConfig := os.Getenv(portDefinitionsEnv)
	if portConfig == "" {
		log.Printf("no port configuration (%s)", portDefinitionsEnv)
		return nil, nil
	}

	portDefinitions := &portDefinitions{}

	err := json.Unmarshal([]byte(strings.Trim(portConfig, "'")), &portDefinitions)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal env data: %s", err)
	}

	return portDefinitions, nil
}
