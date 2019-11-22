package k8s

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"testing"
)


func setEnv(t *testing.T, envFile string) {
	portDefinitions, err := ioutil.ReadFile(envFile)
	require.NoError(t, err)
	err = os.Setenv(portDefinitionsEnv, string(portDefinitions))
	require.NoError(t, err)
}

func unsetEnv(t *testing.T) {
	require.NoError(t, os.Unsetenv(portDefinitionsEnv))
}

func TestPortsFetch(t *testing.T) {
	setEnv(t, "testdata/port_definitions.json")
	defer unsetEnv(t)

	actualPortDef, err := getPortDefinitions()
	require.NoError(t, err)

	servicePortIndex := 0
	assert.Equal(t, 31000, (*actualPortDef)[servicePortIndex].Port)
	assert.Equal(t, true, (*actualPortDef)[servicePortIndex].Labels[0]["service"])

	probePortIndex := 1
	assert.Equal(t, 31001, (*actualPortDef)[probePortIndex].Port)
	assert.Equal(t, true, (*actualPortDef)[probePortIndex].Labels[0]["probe"])

	securedPortIndex := 2
	assert.Equal(t, 31002, (*actualPortDef)[securedPortIndex].Port)
	assert.Equal(t, "generic-app-secured", (*actualPortDef)[securedPortIndex].Labels[0]["consul"])
	assert.Equal(t, "tag", (*actualPortDef)[securedPortIndex].Labels[1]["secureConnection:true"])

	genericPortIndex := 3
	assert.Equal(t, 31003, (*actualPortDef)[genericPortIndex].Port)
	assert.Equal(t, "generic-app", (*actualPortDef)[genericPortIndex].Labels[0]["consul"])
	assert.Equal(t, "tag", (*actualPortDef)[genericPortIndex].Labels[1]["service-port:31000"])
	assert.Equal(t, "tag", (*actualPortDef)[genericPortIndex].Labels[2]["frontend:generic-app"])
	assert.Equal(t, "tag", (*actualPortDef)[genericPortIndex].Labels[3]["envoy"])
}

func TestPortIsService(t *testing.T) {
	setEnv(t, "testdata/port_definitions.json")
	defer unsetEnv(t)

	service := portDefinition{
		Port: 31000,
		Labels: []label{
			{serviceLabel: true},
		},
	}
	assert.True(t, service.isService())
	assert.False(t, service.isProbe())
}

func TestPortIsProbe(t *testing.T) {
	setEnv(t, "testdata/port_definitions.json")
	defer unsetEnv(t)

	service := portDefinition{
		Port: 31000,
		Labels: []label{
			{probeLabel: true},
		},
	}

	assert.True(t, service.isProbe())
	assert.False(t, service.isService())
}

func TestPortConsulName(t *testing.T) {
	setEnv(t, "testdata/port_definitions.json")
	defer unsetEnv(t)

	service := portDefinition{
		Port: 31000,
		Labels: []label{
			{consulLabel: "generic-service"},
		},
	}

	assert.Equal(t, "generic-service", service.labelForConsul())
	assert.False(t, service.isProbe())
	assert.False(t, service.isService())
}
