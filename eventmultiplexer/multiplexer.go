package eventmultiplexer

import (
	"context"
	"log"
	"strings"
	"sync"

	"github.com/soupdiver/creg/types"
)

type DockerEventMultiplexer struct {
	In     []<-chan types.ContainerEventV2
	Out    map[string]chan types.ContainerEventV2
	outMtx sync.RWMutex
}

func New(in ...<-chan types.ContainerEventV2) *DockerEventMultiplexer {
	return &DockerEventMultiplexer{
		In:  in,
		Out: make(map[string]chan types.ContainerEventV2),
	}
}

func (m *DockerEventMultiplexer) NewOutput(backendName string) chan types.ContainerEventV2 {
	c := make(chan types.ContainerEventV2)

	m.outMtx.Lock()
	m.Out[backendName] = c
	m.outMtx.Unlock()

	return c
}

func (m *DockerEventMultiplexer) Run(ctx context.Context) {
	for _, input := range m.In {
		input := input

		go func() {
			for {
				select {
				case <-ctx.Done():
					log.Printf("Multiplexer exiiting: %s", "context cancelled")
					return
				case event, ok := <-input:
					if !ok {
						log.Printf("Multiplexer exiiting: %s", "channel closed")
						return
					}
					for name, cOut := range m.Out {
						// If no backends are defined, send to all
						if v, ok := event.Container.Labels["creg.backends"]; !ok {
							cOut <- event
							continue
						} else {
							// If backends are defined filter for them
							split := strings.Split(v, ",")
						Backends:
							for _, backend := range split {
								if backend == name || backend == "all" {
									cOut <- event
									break Backends
								}
							}
						}
					}
				}
			}
		}()
	}
}
