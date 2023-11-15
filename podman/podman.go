package podman

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/containers/podman/v4/libpod/define"
	"github.com/docker/docker/api/types"
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

type ContainerInspect struct {
	ID string `json:"Id"`
	// Created                 time.Time                          `json:"Created"`
	// Path                    string                             `json:"Path"`
	// Args                    []string                           `json:"Args"`
	State *define.InspectContainerState `json:"State"`
	// Image                   string                             `json:"Image"`
	// ImageDigest             string                             `json:"ImageDigest"`
	// ImageName               string                             `json:"ImageName"`
	// Rootfs                  string                             `json:"Rootfs"`
	// Pod                     string                             `json:"Pod"`
	// ResolvConfPath          string                             `json:"ResolvConfPath"`
	// HostnamePath            string                             `json:"HostnamePath"`
	// HostsPath               string                             `json:"HostsPath"`
	// StaticDir               string                             `json:"StaticDir"`
	// OCIConfigPath           string                             `json:"OCIConfigPath,omitempty"`
	// OCIRuntime              string                             `json:"OCIRuntime,omitempty"`
	// ConmonPidFile           string                             `json:"ConmonPidFile"`
	// PidFile                 string                             `json:"PidFile"`
	// Name                    string                             `json:"Name"`
	// RestartCount            int32                              `json:"RestartCount"`
	// Driver                  string                             `json:"Driver"`
	// MountLabel              string                             `json:"MountLabel"`
	// ProcessLabel            string                             `json:"ProcessLabel"`
	// AppArmorProfile         string                             `json:"AppArmorProfile"`
	// EffectiveCaps           []string                           `json:"EffectiveCaps"`
	// BoundingCaps            []string                           `json:"BoundingCaps"`
	// ExecIDs                 []string                           `json:"ExecIDs"`
	// GraphDriver             *define.DriverData                 `json:"GraphDriver"`
	// SizeRw                  *int64                             `json:"SizeRw,omitempty"`
	// SizeRootFs              int64                              `json:"SizeRootFs,omitempty"`
	// Mounts                  []define.InspectMount              `json:"Mounts"`
	// Dependencies            []string                           `json:"Dependencies"`
	NetworkSettings *define.InspectNetworkSettings `json:"NetworkSettings"`
	// Namespace               string                             `json:"Namespace"`
	// IsInfra                 bool                               `json:"IsInfra"`
	// IsService               bool                               `json:"IsService"`
	// KubeExitCodePropagation string                             `json:"KubeExitCodePropagation"`
	// LockNumber              uint32                             `json:"lockNumber"`
	Config     *CustomConfig                      `json:"Config"`
	HostConfig *define.InspectContainerHostConfig `json:"HostConfig"`
}

type CustomConfig struct {
	// Container hostname
	Hostname string `json:"Hostname"`
	// Container domain name - unused at present
	DomainName string `json:"Domainname"`
	// User the container was launched with
	User string `json:"User"`
	// Unused, at present
	AttachStdin bool `json:"AttachStdin"`
	// Unused, at present
	AttachStdout bool `json:"AttachStdout"`
	// Unused, at present
	AttachStderr bool `json:"AttachStderr"`
	// Whether the container creates a TTY
	Tty bool `json:"Tty"`
	// Whether the container leaves STDIN open
	OpenStdin bool `json:"OpenStdin"`
	// Whether STDIN is only left open once.
	// Presently not supported by Podman, unused.
	StdinOnce bool `json:"StdinOnce"`
	// Container environment variables
	Env []string `json:"Env"`
	// Container command
	Cmd []string `json:"Cmd"`
	// Container image
	Image string `json:"Image"`
	// Unused, at present. I've never seen this field populated.
	Volumes map[string]struct{} `json:"Volumes"`
	// Container working directory
	WorkingDir string `json:"WorkingDir"`
	// Container entrypoint
	// Entrypoint string `json:"Entrypoint"`
	// On-build arguments - presently unused. More of Buildah's domain.
	// OnBuild *string `json:"OnBuild"`
	// Container labels
	Labels map[string]string `json:"Labels"`
	// Container annotations
	Annotations map[string]string `json:"Annotations"`
	// Container stop signal
	// StopSignal uint `json:"StopSignal"`
	// Configured healthcheck for the container
	// Healthcheck *manifest.Schema2HealthConfig `json:"Healthcheck,omitempty"`
	// HealthcheckOnFailureAction defines an action to take once the container turns unhealthy.
	// HealthcheckOnFailureAction string `json:"HealthcheckOnFailureAction,omitempty"`
	// CreateCommand is the full command plus arguments of the process the
	// container has been created with.
	CreateCommand []string `json:"CreateCommand,omitempty"`
	// Timezone is the timezone inside the container.
	// Local means it has the same timezone as the host machine
	Timezone string `json:"Timezone,omitempty"`
	// SystemdMode is whether the container is running in systemd mode. In
	// systemd mode, the container configuration is customized to optimize
	// running systemd in the container.
	SystemdMode bool `json:"SystemdMode,omitempty"`
	// Umask is the umask inside the container.
	Umask string `json:"Umask,omitempty"`
	// Secrets are the secrets mounted in the container
	Secrets []*define.InspectSecret `json:"Secrets,omitempty"`
	// Timeout is time before container is killed by conmon
	Timeout uint `json:"Timeout"`
	// StopTimeout is time before container is stopped when calling stop
	StopTimeout uint `json:"StopTimeout"`
	// Passwd determines whether or not podman can add entries to /etc/passwd and /etc/group
	// Passwd *bool `json:"Passwd,omitempty"`
	// ChrootDirs is an additional set of directories that need to be
	// treated as root directories. Standard bind mounts will be mounted
	// into paths relative to these directories.
	ChrootDirs []string `json:"ChrootDirs,omitempty"`
	// SdNotifyMode is the sd-notify mode of the container.
	// SdNotifyMode string `json:"sdNotifyMode,omitempty"`
	// SdNotifySocket is the NOTIFY_SOCKET in use by/configured for the container.
	// SdNotifySocket string `json:"sdNotifySocket,omitempty"`
}

func (c *PodmanEventsClient) GetEventsForCreg(ctx context.Context, label string) chan ctypes.ContainerEventV2 {
	log := ctx.Value("log").(*logrus.Entry).WithField("source", "podman")
	eventsChannel := make(chan ctypes.ContainerEventV2)

	go func() {
		time.Sleep(1 * time.Second)

		rContainers, err := GetRunningContainers()
		if err != nil {
			log.Errorf("Error getting running containers: %s", err)
			return
		}

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
			// log.Debugf("Sending startup event: %+v", e)
			eventsChannel <- e
		}
	}()

	go func() {
		defer log.Infof("Done reading events")

		path := c.SocketPath
		u := url.URL{Scheme: "http", Path: "/v1.40/events"}

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

		log.Debugf("Ready for events: %s", path)

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			// log.Debug("Reading event")
			var event Event
			if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
				log.Errorf("Error unmarshalling event: %s", err)
				continue
			}

			if event.Action != "start" && event.Action != "stop" {
				continue
			}

			// log.Debugf("Received event: Type=%s, Action=%s, ID=%s\n", event.Type, event.Action, event.Actor.ID)
			ccc := ContainerInfoFromEvent(ctx, event)
			// log.Debugf("ccc: %+v", ccc)
			eventsChannel <- ctypes.ContainerEventV2{
				Action:    event.Action,
				Container: ccc,
			}
			// log.Debugf("Event sent!")
		}
		if err := scanner.Err(); err != nil {
			log.Debugf("Error reading events: %s", err)
		}
		log.Printf("Done reading events!")
	}()

	return eventsChannel
}

func ContainerInfoFromEvent(ctx context.Context, event Event) ctypes.ContainerInfo {
	container, err := GetContainerConfig(ctx, event.Actor.ID)
	if err != nil {
		log.Printf("Error getting container config: %s", err)
		return ctypes.ContainerInfo{}
	}
	// log.Printf("container: %+v", container)

	ci := ctypes.ContainerInfo{
		ID:     event.Actor.ID,
		Labels: map[string]string{},
		NetworkSettings: ctypes.NetworkSettings{
			Ports: map[ctypes.Port][]ctypes.PortBinding{},
		},
	}

	if container.Config != nil {
		ci.Labels = container.Config.Labels
	}

	if container.NetworkSettings != nil {
		// convert container.ExposedPorts to ctypes.Port
		for k, v := range container.NetworkSettings.Ports {
			p := ctypes.Port(k)
			ci.NetworkSettings.Ports[p] = []ctypes.PortBinding{
				{
					HostIP:   v[0].HostIP,
					HostPort: v[0].HostPort,
				},
			}
		}
	}

	// log.Printf("ci: %+v", ci)

	return ci
}

func GetContainerConfig(ctx context.Context, id string) (*types.ContainerJSON, error) {
	path := "/run/podman/podman.sock"
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

	var container types.ContainerJSON

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(body, &container)
	if err != nil {
		return nil, err
	}

	return &container, nil
}

func GetRunningContainers() ([]types.Container, error) {
	path := "/run/podman/podman.sock"
	u := url.URL{Scheme: "http", Path: "/v1.40/containers/json"}

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

	var container []types.Container
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// log.Printf("body: %s", body)
	err = json.Unmarshal(body, &container)
	if err != nil {
		return nil, err
	}

	return container, nil
}
