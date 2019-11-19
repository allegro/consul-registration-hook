package k8s

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

const (
	portDefinitionsEnv = "PORT_DEFINITIONS"
	probeLabel         = "probe"
	serviceLabel       = "service"
	consulLabel        = "consul"
)

type portDefinitions []portDefinition

type portDefinition struct {
	Port   int     `json:"port"`
	Labels []label `json:"labels"`
}

type label map[string]interface{}

func (pd portDefinition) getTags() []string {
	var tags []string
	for _, label := range pd.Labels {
		for key, value := range label {
			labelValue, ok := value.(string)
			if ok {
				if labelValue == "tag" {
					tags = append(tags, key)
				}
			}
		}
	}
	return tags
}

func (pd portDefinition) isService() bool {
	for i := range pd.Labels {
		if pd.hasServiceLabel(i) {
			return true
		}
	}
	return false
}

func (pd portDefinition) isProbe() bool {
	for i := range pd.Labels {
		if pd.hasProbeLabel(i) {
			return true
		}
	}
	return false
}

func (pd portDefinition) labelForConsul() string {
	for i := range pd.Labels {
		if pd.hasConsulLabel(i) {
			return pd.Labels[i][consulLabel].(string)
		}
	}
	return ""
}

func (pd portDefinition) hasConsulLabel(labelIndex int) bool {
	label := pd.Labels[labelIndex]
	if _, ok := label[consulLabel]; ok {
		return true
	}

	return false
}

func (pd portDefinition) hasServiceLabel(labelIndex int) bool {
	label := pd.Labels[labelIndex]
	if val, ok := label[serviceLabel]; ok {
		return val.(bool)
	}

	return false
}

func (pd portDefinition) hasProbeLabel(labelIndex int) bool {
	label := pd.Labels[labelIndex]
	if val, ok := label[probeLabel]; ok {
		return val.(bool)
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

	err := json.Unmarshal([]byte(portConfig), &portDefinitions)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal env data: %s", err)
	}

	return portDefinitions, nil
}
