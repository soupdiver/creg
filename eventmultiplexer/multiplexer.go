package eventmultiplexer

import (
	"github.com/soupdiver/creg/docker"
)

type DockerEventMultiplexer struct {
	In  <-chan docker.ContainerEvent
	Out []chan docker.ContainerEvent
}

func New(in <-chan docker.ContainerEvent) *DockerEventMultiplexer {
	return &DockerEventMultiplexer{
		In:  in,
		Out: []chan docker.ContainerEvent{},
	}
}

func (m *DockerEventMultiplexer) NewOutput() chan docker.ContainerEvent {
	c := make(chan docker.ContainerEvent, 5)

	m.Out = append(m.Out, c)

	return c
}

func (m *DockerEventMultiplexer) Run() {
	go func() {
		for event := range m.In {
			for _, backend := range m.Out {
				backend <- event
			}
		}
	}()
}
