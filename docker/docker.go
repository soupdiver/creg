package docker

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/sirupsen/logrus"

	ctypes "github.com/soupdiver/creg/types"
)

type ContainerEvent struct {
	Event     ctypes.ContainerEvent
	Container types.ContainerJSON
}

func GetContainersForCreg(ctx context.Context, client *client.Client, label string) ([]ctypes.ContainerInfo, error) {
	containers, err := client.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		return nil, err
	}

	cregContainers := []ctypes.ContainerInfo{}
	for _, container := range containers {
		if label != "" {
			if v, ok := container.Labels[label]; !ok || v != "true" {
				continue
			}
		}

		container, err := client.ContainerInspect(ctx, container.ID)
		if err != nil {
			return nil, err
		}

		v := ConvertContainerFromDocker(container)
		cregContainers = append(cregContainers, v)
	}

	return cregContainers, nil
}

func GetEventsForCreg(ctx context.Context, client *client.Client, label string) chan ctypes.ContainerEventV2 {
	log := ctx.Value("log").(*logrus.Entry).WithField("backend", "docker")

	ctx, cancel := context.WithCancel(ctx)
	es, cerr := client.Events(ctx, types.EventsOptions{})
	go func() {
		for err := range cerr {
			log.Errorf("Error receiving events: %s", err)
		}
		cancel()
	}()

	c := make(chan ctypes.ContainerEventV2)

	go func() {
		for {

			select {
			case <-ctx.Done():
				return
			case event := <-es:
				log.Debugf("Received Event: %+v for container: %s", event.Action, event.Actor.ID)
				switch event.Action {
				case "start", "stop":
					container, err := client.ContainerInspect(ctx, event.Actor.ID)
					if err != nil {
						log.Errorf("Error inspecting container: %s", err)
						continue
					}

					if label != "" {
						if v, ok := container.Config.Labels[label]; !ok || v != "true" {
							// log.Printf("discard label %s: %+v", label, v)
							continue
						}
					}

					c <- ctypes.ContainerEventV2{
						Action:    string(event.Action),
						Container: ConvertContainerFromDocker(container),
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

func ConvertContainerFromDocker(in types.ContainerJSON) ctypes.ContainerInfo {
	return ctypes.ContainerInfo{
		ID:              in.ID,
		Labels:          in.Config.Labels,
		NetworkSettings: ConvertNetworkSettingsFromDocker(in.NetworkSettings),
	}
}

func ConvertNetworkSettingsFromDocker(in *types.NetworkSettings) ctypes.NetworkSettings {
	v := ctypes.NetworkSettings{
		Ports: make(map[ctypes.Port][]ctypes.PortBinding),
	}

	for port, info := range in.Ports {
		v.Ports[ctypes.Port(port.Port()+"/"+port.Proto())] = []ctypes.PortBinding{
			{
				HostIP:   info[0].HostIP,
				HostPort: info[0].HostPort,
			},
		}
	}

	return v
}
