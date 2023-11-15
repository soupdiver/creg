package docker

import (
	"context"
	"log"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
	ctypes "github.com/soupdiver/creg/types"
)

type ContainerEvent struct {
	Event     events.Message
	Container types.ContainerJSON
}

func GetContainersForCreg(ctx context.Context, client *client.Client, label string) ([]types.ContainerJSON, error) {
	containers, err := client.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		return nil, err
	}

	cregContainers := []types.ContainerJSON{}
	for _, container := range containers {
		if v, ok := container.Labels[label]; !ok || v != "true" {
			// log.Printf("label: %+v", container.Labels[label])
			continue
		}
		// log.Printf("found container: %s", container.ID)

		container, err := client.ContainerInspect(ctx, container.ID)
		if err != nil {
			return nil, err
		}

		cregContainers = append(cregContainers, container)
	}

	return cregContainers, nil
}

func GetEventsForCreg(ctx context.Context, client *client.Client, label string) chan ctypes.ContainerEvent {
	ctx, cancel := context.WithCancel(ctx)
	es, cerr := client.Events(ctx, types.EventsOptions{})
	go func() {
		for err := range cerr {
			log.Printf("Error reading docker events: %s", err)
		}
		cancel()
	}()

	c := make(chan ctypes.ContainerEvent)

	go func() {
		for {
			select {
			case <-ctx.Done():
				// close(c)
			case event := <-es:
				switch event.Action {
				case "start", "stop":
					container, err := client.ContainerInspect(ctx, event.Actor.ID)
					if err != nil {
						log.Printf("err inspect: %s", err)
						continue
					}

					// if v, ok := container.Config.Labels[label]; ok {
					// log.Printf("found label %s: %s", label, v)
					// }

					if v, ok := container.Config.Labels[label]; !ok || v != "true" {
						// log.Printf("discard label %s: %+v", label, v)
						continue
					}
					// log.Printf("forward label %s: %+v", label, container.Config.Labels[label])

					c <- ctypes.ContainerEvent{
						Event:     event,
						Container: container,
					}
				}
			}
		}
	}()

	return c
}

func ContainerInfoFromEvent(event ctypes.ContainerEvent) ctypes.ContainerInfo {
	return ctypes.ContainerInfo{
		ID:     event.Container.ID,
		Labels: event.Container.Config.Labels,
	}
}
