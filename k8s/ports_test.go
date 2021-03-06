package k8s

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testFiles = []string{"testdata/port_definitions.json", "testdata/port_definitions_quoted.json"}

func TestPortsFetch(t *testing.T) {
	for _, envFile := range testFiles {
		setEnv(t, envFile)
		actualPortDef, err := getPortDefinitions()
		if err != nil {
			t.Fatalf("%s for %s", err, envFile)
		}

		servicePortIndex := 0
		assert.Equal(t, 31000, (*actualPortDef)[servicePortIndex].Port)
		assert.Equal(t, "tag", (*actualPortDef)[servicePortIndex].Labels["envoy"])

		probePortIndex := 1
		assert.Equal(t, 31001, (*actualPortDef)[probePortIndex].Port)
		assert.Equal(t, "true", (*actualPortDef)[probePortIndex].Labels["probe"])

		securedPortIndex := 2
		assert.Equal(t, 31002, (*actualPortDef)[securedPortIndex].Port)
		assert.Equal(t, "generic-app-secured", (*actualPortDef)[securedPortIndex].Labels["consul"])
		assert.Equal(t, "tag", (*actualPortDef)[securedPortIndex].Labels["secureConnection:true"])

		genericPortIndex := 3
		assert.Equal(t, 31003, (*actualPortDef)[genericPortIndex].Port)
		assert.Equal(t, "generic-app", (*actualPortDef)[genericPortIndex].Labels["consul"])
		assert.Equal(t, "tag", (*actualPortDef)[genericPortIndex].Labels["service-port:31000"])
		assert.Equal(t, "tag", (*actualPortDef)[genericPortIndex].Labels["frontend:generic-app"])
	}
	defer unsetEnv(t)
}

func TestPortIsService(t *testing.T) {
	service := portDefinition{
		Port: 31000,
		Labels: label{serviceLabel: "true"},

	}
	assert.True(t, service.isService())
	assert.False(t, service.isProbe())
}

func TestPortIsProbe(t *testing.T) {
	service := portDefinition{
		Port: 31000,
		Labels: label{probeLabel: "true"},
	}

	assert.True(t, service.isProbe())
	assert.False(t, service.isService())
}

func TestPortConsulName(t *testing.T) {
	service := portDefinition{
		Port:   31000,
		Labels: label{consulLabel: "generic-service"},
	}

	assert.Equal(t, "generic-service", service.labelForConsul())
	assert.False(t, service.isProbe())
	assert.False(t, service.isService())
}

func setEnv(t *testing.T, envFile string) {
	portDefinitions, err := ioutil.ReadFile(envFile)
	require.NoError(t, err)
	err = os.Setenv(portDefinitionsEnv, string(portDefinitions))
	require.NoError(t, err)
}

func unsetEnv(t *testing.T) {
	require.NoError(t, os.Unsetenv(portDefinitionsEnv))
}
