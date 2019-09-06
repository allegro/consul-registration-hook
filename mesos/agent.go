package mesos

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

const (
	defaultAgentBaseURL = "http://localhost:5050"
	stateEndpointFormat = "%s/state"
)

type label struct {
	Key   string
	Value string
}

type labels struct {
	Labels []label
}

type port struct {
	Number   int
	Protocol string
	Name     string
	Labels   labels
}

type ports struct {
	Ports []port
}

type discovery struct {
	Ports ports
}

type task struct {
	ID        string
	Labels    []label
	Discovery discovery
}

type executor struct {
	ID    string
	Tasks []task
}

type framework struct {
	ID        string
	Executors []executor
}

type state struct {
	Frameworks []framework
}

type agentClient interface {
	state() (state, error)
}

type defaultAgentClient struct {
	baseURL string
}

func (ac defaultAgentClient) state() (state, error) {
	state := state{}

	url := fmt.Sprintf(stateEndpointFormat, ac.baseURL)
	resp, err := http.Get(url)
	if err != nil {
		return state, fmt.Errorf("unable connect to mesos agent: %s", err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return state, fmt.Errorf("unable read response from mesos agent: %s", err)
	}

	if err := json.Unmarshal(body, &state); err != nil {
		return state, fmt.Errorf("unable to unmarshal mesos agent state response: %s", err)
	}

	return state, nil
}
