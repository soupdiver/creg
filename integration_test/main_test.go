package integratontest_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"testing"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/hashicorp/consul/api"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	clientv3 "go.etcd.io/etcd/client/v3"
)

var (
	cli          *client.Client
	consulClient *api.Client
	EtcdClient   *clientv3.Client
)

func HelperRemoveContainer(cli *client.Client, containerName string) error {
	// remove container
	err := cli.ContainerRemove(context.Background(), containerName, dockertypes.ContainerRemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})
	if err != nil {
		return fmt.Errorf("could not remove container %s: %w", containerName, err)
	}

	return nil
}

func TestEtcdlStartAndStop(t *testing.T) {
	t.Parallel()
	// create docker container with nginx image
	resp, err := cli.ContainerCreate(context.Background(), &container.Config{
		Image: "nginx",
		Labels: map[string]string{
			"creg":          "true",
			"creg.ports":    "'80/tcp:etcd-nginx'",
			"creg.backends": "etcd",
		},
	}, nil, nil, &v1.Platform{Architecture: "amd64"}, "etcd-nginx-02")
	defer func() {
		HelperRemoveContainer(cli, resp.ID)
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
	time.Sleep(2 * time.Second)

	prefix := "creg/etcd-nginx"

	// serviceKey := etcd.GenerateServiceKey("etcd-nginx")
	// serviceKeys := strings.Split(serviceKey, "/")
	// serviceKey = strings.Join(serviceKeys[:len(serviceKeys)-1], "/")

	// t.Logf("lookup: %s", serviceKey)

	rangeEnd := clientv3.GetPrefixRangeEnd(prefix)

	s, err := EtcdClient.Get(context.Background(), prefix, clientv3.WithRange(rangeEnd))
	if err != nil {
		t.Fatal(err)
	}

	// we expect one more service than before
	if s.Count != 1 {
		t.Fatalf("expected %d services, got %d", 1, s.Count)
	}

	// stop container
	err = cli.ContainerStop(context.Background(), resp.ID, container.StopOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// give some time for the container to stop
	time.Sleep(1 * time.Second)

	// // remove container
	// err = cli.ContainerRemove(context.Background(), resp.ID, dockertypes.ContainerRemoveOptions{
	// 	Force:         true,
	// 	RemoveVolumes: true,
	// })
	// if err != nil {
	// 	t.Fatal(err)
	// }

	// give some time for the container to stop
	time.Sleep(1 * time.Second)

	s, err = EtcdClient.Get(context.Background(), prefix, clientv3.WithRange(rangeEnd))
	if err != nil {
		t.Fatal(err)
	}
	t.Log(54)

	// we expect one less service than before
	if s.Count != 0 {
		t.Fatalf("expected %d services, got %d", 0, s.Count)
	}
}

func TestConsulStartAndStop(t *testing.T) {
	t.Parallel()
	// create docker container with nginx image
	resp, err := cli.ContainerCreate(context.Background(), &container.Config{
		Image: "nginx",
		Labels: map[string]string{
			"creg":          "true",
			"creg.ports":    "'80/tcp:consul-nginx'",
			"creg.backends": "consul",
		},
	}, nil, nil, &v1.Platform{Architecture: "amd64"}, "consul-nginx-01")
	defer func() {
		HelperRemoveContainer(cli, resp.ID)
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
	time.Sleep(2 * time.Second)

	s, err := consulClient.Agent().ServicesWithFilter("Service == \"creg-consul-nginx\"")
	if err != nil {
		t.Fatal(err)
	}

	// we expect one more service than before
	if len(s) != 1 {
		t.Fatalf("expected %d services, got %d", 1, len(s))
	}

	// stop container
	err = cli.ContainerStop(context.Background(), resp.ID, container.StopOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// give some time for the container to stop
	time.Sleep(1 * time.Second)

	// // remove container
	// err = cli.ContainerRemove(context.Background(), resp.ID, dockertypes.ContainerRemoveOptions{
	// 	Force:         true,
	// 	RemoveVolumes: true,
	// })
	// if err != nil {
	// 	t.Fatal(err)
	// }

	// give some time for the container to stop
	time.Sleep(1 * time.Second)

	s, err = consulClient.Agent().ServicesWithFilter("Service == \"creg-consul-nginx\"")
	if err != nil {
		t.Fatal(err)
	}

	// we expect one less service than before
	if len(s) != 0 {
		t.Fatalf("expected %d services, got %d", 0, len(s))
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

	time.Sleep(1 * time.Second)

	consulClient, err = api.NewClient(api.DefaultConfig())
	if err != nil {
		panic(err)
	}

	EtcdClient, err = clientv3.New(clientv3.Config{
		Endpoints:   []string{"http://localhost:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		panic(err)
	}

	log.Print(1)

	// run tests
	code := m.Run()

	// fetch logs from integration_test-creg-1
	logs, err := cli.ContainerLogs(context.Background(), "integration_test-creg-1", dockertypes.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		log.Print(err)
	} else {
		logsBytes, err := ioutil.ReadAll(logs)
		if err != nil {
			log.Print(err)
		}
		log.Printf("logs:\n\n\n%s\n\n", string(logsBytes))
	}

	// shutdown
	err = applyDockerCompose("./docker-compose.yml", "down")
	if err != nil {
		panic(err)
	}

	os.Exit(code)
}

func applyDockerCompose(composeFile string, args ...string) error {
	argz := []string{"compose", "-f", composeFile}
	argz = append(argz, args...)
	cmd := exec.Command("docker", argz...)
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run docker-compose: %w", err)
	}

	return nil
}
