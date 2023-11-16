package podman

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/url"

	"github.com/sirupsen/logrus"
	ctypes "github.com/soupdiver/creg/types"
)

type PodmanEventsClient struct {
	SocketPath string
}

func NewPodmanEventsClient(path string) *PodmanEventsClient {
	return &PodmanEventsClient{
		SocketPath: path,
	}
}

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

func (c *PodmanEventsClient) GetEventsForCreg(ctx context.Context, label string) chan ctypes.ContainerEventV2 {
	log := ctx.Value("log").(*logrus.Entry).WithField("backend", "podman")
	eventsChannel := make(chan ctypes.ContainerEventV2)

	go func() {
		defer log.Infof("Done reading events")

		path := c.SocketPath
		u := url.URL{Scheme: "http", Path: "/v1.40/events"}

		log.Debugf("Connecting to Podman socket: %s", path)

		conn, err := net.Dial("unix", path)
		if err != nil {
			log.Errorf("Error connecting to Podman socket: %s", err)
			return
		}
		defer conn.Close()

		req, err := http.NewRequest("GET", u.String(), nil)
		if err != nil {
			log.Errorf("Error creating request: %s", err)
			return
		}

		err = req.Write(conn)
		if err != nil {
			log.Errorf("Error writing request: %s", err)
			return
		}

		resp, err := http.ReadResponse(bufio.NewReader(conn), req)
		if err != nil {
			log.Errorf("Error reading response: %s", err)
			return
		}
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			var event Event
			if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
				log.Errorf("Error unmarshalling event: %s", err)
				return
			}
			log.Debugf("Received event: Type=%s, Action=%s, ID=%s\n", event.Type, event.Action, event.Actor.ID)
		}
		if err := scanner.Err(); err != nil {
			log.Debugf("Error reading events: %s", err)
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

func GetContainerConfig(ctx context.Context, id string) (*Config, error) {
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
