package docker

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
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

func GetEventsForCreg(ctx context.Context, client *client.Client, enableLabel, cregID string) chan ctypes.ContainerEventV2 {
	log := ctx.Value("log").(*logrus.Entry).WithField("source", "docker")

	c := make(chan ctypes.ContainerEventV2)

	go func() {
		time.Sleep(1 * time.Second)
		rContainers, err := GetRunningContainers(ctx, client)
		if err != nil {
			log.Errorf("Error getting running containers: %s", err)
		} else {
			for _, container := range rContainers {
				e := ctypes.ContainerEventV2{
					Action: "start",
					Container: ctypes.ContainerInfo{
						ID:     container.ID,
						Labels: container.Labels,
						NetworkSettings: ctypes.NetworkSettings{
							Ports: map[ctypes.Port][]ctypes.PortBinding{},
						},
					},
				}
				for _, v := range container.Ports {
					p := ctypes.Port(fmt.Sprintf("%d/%s", v.PrivatePort, v.Type))
					e.Container.NetworkSettings.Ports[p] = []ctypes.PortBinding{
						{
							HostIP:   v.IP,
							HostPort: fmt.Sprintf("%d", v.PublicPort),
						},
					}
				}
				c <- e
			}
		}
	}()

	ctx, cancel := context.WithCancel(ctx)
	es, cerr := client.Events(ctx, types.EventsOptions{})
	go func() {
		for err := range cerr {
			log.Errorf("Error receiving events: %s", err)
		}
		cancel()
	}()

	go func() {
		log.Debugf("Ready for events")
		for {

			select {
			case <-ctx.Done():
				return
			case event := <-es:
				log.Debugf("Received Event: %+v for container: %s - %s", event.Action, event.Actor.Attributes["name"], event.Actor.ID)
				switch event.Action {
				case "start", "kill":
					container, err := client.ContainerInspect(ctx, event.Actor.ID)
					if err != nil {
						log.Errorf("Error inspecting container: %s", err)
						continue
					}

					// When empty handle all containers
					if enableLabel != "" {
						// label must be set and be true
						if v, ok := container.Config.Labels[enableLabel]; !ok || v != "true" {
							// log.Printf("discard label %s: %+v", label, v)
							continue
						}

						// creg.id if set must match
						if v, ok := container.Config.Labels["creg.id"]; ok && v != cregID {
							log.Printf("discard creg.id %s: %+v", cregID, v)
							continue
						}
					}

					c <- ctypes.ContainerEventV2{
						Action:    string(event.Action),
						Container: ConvertContainerFromDocker(container),
					}

				default:
					log.Debugf("Discard Event: %+v for container: %s", event.Action, event.Actor.ID)
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
		if len(info) > 0 {
			v.Ports[ctypes.Port(port.Port()+"/"+port.Proto())] = []ctypes.PortBinding{
				{
					HostIP:   info[0].HostIP,
					HostPort: info[0].HostPort,
				},
			}
		}
	}

	return v
}

func GetRunningContainers(ctx context.Context, client *client.Client) ([]types.Container, error) {
	args := filters.NewArgs()
	args.Add("status", "running")

	containers, err := client.ContainerList(ctx, container.ListOptions{
		Filters: args,
	})
	if err != nil {
		return nil, err
	}

	return containers, nil
}
