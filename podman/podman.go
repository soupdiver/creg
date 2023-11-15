package podman

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"

	"github.com/docker/docker/client"
	ctypes "github.com/soupdiver/creg/types"
)

type Event struct {
	Type   string `json:"Type"`
	Action string `json:"Action"`
	Actor  struct {
		ID string `json:"ID"`
	} `json:"Actor"`
}

type Container struct {
	ID     string `json:"Id"`
	Config Config `json:"Config"`
}

type Config struct {
	Hostname     string              `json:"Hostname"`
	Domainname   string              `json:"Domainname"`
	User         string              `json:"User"`
	AttachStdin  bool                `json:"AttachStdin"`
	AttachStdout bool                `json:"AttachStdout"`
	AttachStderr bool                `json:"AttachStderr"`
	ExposedPorts map[string]struct{} `json:"ExposedPorts"`
	Tty          bool                `json:"Tty"`
	OpenStdin    bool                `json:"OpenStdin"`
	StdinOnce    bool                `json:"StdinOnce"`
	Env          []string            `json:"Env"`
	Cmd          []string            `json:"Cmd"`
	Image        string              `json:"Image"`
	Volumes      map[string]struct{} `json:"Volumes"`
	WorkingDir   string              `json:"WorkingDir"`
	Entrypoint   []string            `json:"Entrypoint"`
	OnBuild      []string            `json:"OnBuild"`
	Labels       map[string]string   `json:"Labels"`
}

func GetEventsForCreg(ctx context.Context, client *client.Client, label string) chan ctypes.ContainerEventV2 {
	resp, err := http.Get("unix:///var/run/docker.sock/v1.40/events")
	if err != nil {
		fmt.Println(err)
		return nil
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)

	eventsChannel := make(chan ctypes.ContainerEventV2)
	go func() {
		for scanner.Scan() {
			var event Event
			if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
				fmt.Println(err)
				continue
			}
			fmt.Printf("Received event: Type=%s, Action=%s, ID=%s\n", event.Type, event.Action, event.Actor.ID)

			cfg, err := GetContainerConfig(ctx, client, event.Actor.ID)
			if err != nil {
				fmt.Println(err)
				continue
			}

			eventsChannel <- ctypes.ContainerEventV2{
				Container: ctypes.ContainerInfo{
					ID:     event.Actor.ID,
					Labels: cfg.Labels,
				},
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Println(err)
			close(eventsChannel)
		}
	}()

	return eventsChannel
}

func ContainerInfoFromEvent(event Event) ctypes.ContainerInfo {
	return ctypes.ContainerInfo{
		ID:     event.Actor.ID,
		Labels: map[string]string{},
	}
}

func GetContainerConfig(ctx context.Context, client *client.Client, id string) (*Config, error) {
	path := "/var/run/docker.sock"
	u := url.URL{Scheme: "http", Path: "/v1.40/containers/" + id + "/json"}

	conn, err := net.Dial("unix", path)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	err = req.Write(conn)
	if err != nil {
		return nil, err
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var container Container
	if err := json.NewDecoder(resp.Body).Decode(&container); err != nil {
		return nil, err
	}

	return &container.Config, nil
}
