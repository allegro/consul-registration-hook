package mesos

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIfReturnsAgentState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		state, err := ioutil.ReadFile("testdata/state.json")
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
		rw.Write(state)
	}))
	defer server.Close()

	agentClient := defaultAgentClient{
		baseURL: server.URL,
	}

	state, err := agentClient.state()

	require.NoError(t, err)
	require.Len(t, state.Frameworks, 1)
	require.Len(t, state.Frameworks[0].Executors, 1)
	require.Len(t, state.Frameworks[0].Executors[0].Tasks, 1)
}

func TestIfReturnsErrorWhenAgentUnavailable(t *testing.T) {
	agentClient := defaultAgentClient{
		baseURL: "http://192.0.2.2:5050",
	}

	_, err := agentClient.state()

	assert.Error(t, err)
}
