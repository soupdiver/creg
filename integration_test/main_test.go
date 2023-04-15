package integratontest_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"

	"github.com/hashicorp/consul/api"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

var (
	cli          *client.Client
	consulClient *api.Client
)

func TestCreate(t *testing.T) {
	s, err := consulClient.Agent().Services()
	if err != nil {
		t.Fatal(err)
	}
	cStart := len(s)

	// create docker container with nginx image
	resp, err := cli.ContainerCreate(context.Background(), &container.Config{
		Image:  "nginx",
		Labels: map[string]string{"creg": "true", "creg.ports": "'80/tcp:nginx'"},
	}, nil, nil, &v1.Platform{Architecture: "amd64"}, "nginx-01")
	defer func() {
		err = cli.ContainerRemove(context.Background(), resp.ID, dockertypes.ContainerRemoveOptions{
			Force:         true,
			RemoveVolumes: true,
		})
		if err != nil {
			t.Log(err)
		}
	}()
	if err != nil {
		t.Fatal(err)
	}

	// start container
	err = cli.ContainerStart(context.Background(), resp.ID, dockertypes.ContainerStartOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// give some time for the container to start
	time.Sleep(1 * time.Second)

	// // get logs from container named creg
	// logs, err := cli.ContainerLogs(context.Background(), "creg", dockertypes.ContainerLogsOptions{
	// 	ShowStdout: true,
	// 	ShowStderr: true,
	// })
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// txt, err := ioutil.ReadAll(logs)
	// if err == nil {
	// 	log.Printf("logs: %s", txt)
	// }

	s, err = consulClient.Agent().Services()
	if err != nil {
		t.Fatal(err)
	}

	if len(s) != cStart+1 {
		t.Fatalf("expected %d services, got %d", cStart+1, len(s))
	}
}

func TestMain(m *testing.M) {
	// setup
	var err error
	cli, err = client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err)
	}
	cli.NegotiateAPIVersion(context.Background())

	// start clean
	err = applyDockerCompose("./docker-compose.yml", "down")
	if err != nil {
		panic(err)
	}

	err = applyDockerCompose("./docker-compose.yml", "up", "-d")
	if err != nil {
		panic(err)
	}

	time.Sleep(3 * time.Second)

	consulClient, err = api.NewClient(api.DefaultConfig())
	if err != nil {
		panic(err)
	}

	// run tests
	code := m.Run()

	// shutdown
	err = applyDockerCompose("./docker-compose.yml", "down")
	if err != nil {
		panic(err)
	}

	os.Exit(code)
}

func applyDockerCompose(composeFile string, args ...string) error {
	argz := []string{"compose"}
	argz = append(argz, args...)
	cmd := exec.Command("docker", argz...)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run docker-compose: %w", err)
	}

	return nil
}
