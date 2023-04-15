package eventmultiplexer

import (
	"context"
	"log"

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

func (m *DockerEventMultiplexer) Run(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Printf("Multiplexer exiiting: %s", "context cancelled")
				return
			case event, ok := <-m.In:
				if !ok {
					log.Printf("Multiplexer exiiting: %s", "channel closed")
					return
				}
				for _, backend := range m.Out {
					backend <- event
				}
			}
		}
	}()
}
