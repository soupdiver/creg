package eventmultiplexer

import (
	"context"
	"log"
	"strings"
	"sync"

	"github.com/soupdiver/creg/docker"
)

type DockerEventMultiplexer struct {
	In     <-chan docker.ContainerEvent
	Out    map[string]chan docker.ContainerEvent
	outMtx sync.RWMutex
}

func New(in <-chan docker.ContainerEvent) *DockerEventMultiplexer {
	return &DockerEventMultiplexer{
		In:  in,
		Out: make(map[string]chan docker.ContainerEvent),
	}
}

func (m *DockerEventMultiplexer) NewOutput(backendName string) chan docker.ContainerEvent {
	c := make(chan docker.ContainerEvent, 5)

	m.outMtx.Lock()
	m.Out[backendName] = c
	m.outMtx.Unlock()

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
				for name, cOut := range m.Out {
				Labels:
					for k, v := range event.Container.Config.Labels {
						if k == "creg.backends" {
							split := strings.Split(v, ",")
							for _, backend := range split {
								if backend == name || backend == "all" {
									cOut <- event
									break Labels
								}
							}
							event.Container.Config.Labels["creg"] = v
						}
					}
				}
			}
		}
	}()
}
