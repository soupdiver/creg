package docker

import (
	"context"
	"log"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
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
		log.Printf("found container: %s", container.ID)

		container, err := client.ContainerInspect(ctx, container.ID)
		if err != nil {
			return nil, err
		}

		cregContainers = append(cregContainers, container)
	}

	return cregContainers, nil
}

func GetEventsForCreg(ctx context.Context, client *client.Client, label string) chan ContainerEvent {
	ctx, cancel := context.WithCancel(ctx)
	es, cerr := client.Events(ctx, types.EventsOptions{})
	go func() {
		for err := range cerr {
			log.Printf("Error reading docker events: %s", err)
		}
		cancel()
	}()

	c := make(chan ContainerEvent)

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

					if v, ok := container.Config.Labels[label]; !ok || v != "true" {
						continue
					}

					// log.Printf("Foward %s for %s", event.Action, event.Actor.ID)

					c <- ContainerEvent{
						Event:     event,
						Container: container,
					}
				}
			}
		}
	}()

	return c
}
