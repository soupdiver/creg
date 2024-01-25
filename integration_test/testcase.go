package integratontest

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/pkg/errors"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

type TestCase struct {
	Name            string
	Args            []string
	Run             func(t *testing.T, ctx *TestContext)
	network         *testcontainers.DockerNetwork
	cregContainer   testcontainers.Container
	consulContainer testcontainers.Container
}

type TestContext struct {
	CregContainerName   string
	Network             string
	ConsulContainerName string
	ConsulPort          string
}

func (tc *TestCase) Setup() (TestContext, error) {
	ctx := context.Background()

	newNetwork, err := network.New(ctx)
	if err != nil {
		return TestContext{}, fmt.Errorf("failed to create docker network: %w", err)
	}
	tc.network = newNetwork

	// Start Consul container
	consulReq := testcontainers.ContainerRequest{
		// Name:         "consull",
		Image:        "consul:1.15",
		ExposedPorts: []string{"8500/tcp"},
		WaitingFor:   wait.ForListeningPort("8500/tcp"),
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.NetworkMode = container.NetworkMode(newNetwork.ID)
		},
	}
	consulContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: consulReq,
		Started:          true,
	})
	if err != nil {
		return TestContext{}, fmt.Errorf("failed to start consul container: %w", err)
	}
	tc.consulContainer = consulContainer

	// for each string in tc.Args string replace CONSUL_CONTAINER with name of consul container
	consulContainerIP, err := consulContainer.ContainerIP(ctx)
	if err != nil {
		return TestContext{}, fmt.Errorf("failed to get container IP: %w", err)
	}
	consulContainerIP = strings.Trim(consulContainerIP, "/")

	for i, arg := range tc.Args {
		tc.Args[i] = strings.Replace(arg, "CONSUL_CONTAINER", consulContainerIP, -1)
	}

	pwd, err := os.Getwd()
	if err != nil {
		return TestContext{}, fmt.Errorf("failed to get working directory: %w", err)
	}

	// Start Creg container with provided arguments
	cregReq := testcontainers.ContainerRequest{
		// Name:  "creg",
		Image: "alpine",
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.NetworkMode = container.NetworkMode(newNetwork.ID)
			hc.Mounts = []mount.Mount{
				{
					Type:   mount.TypeBind,
					Source: pwd + "/../dist/creg_linux_amd64_v1/creg",
					Target: "/creg",
				},
				{
					Type:     mount.TypeBind,
					Source:   "/var/run/docker.sock",
					Target:   "/var/run/docker.sock",
					ReadOnly: true,
				},
			}
		},
		Cmd: append([]string{"/creg"}, tc.Args...),
	}
	cregContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: cregReq,
		Started:          true,
	})
	if err != nil {
		return TestContext{}, fmt.Errorf("failed to start creg container: %w", err)
	}
	tc.cregContainer = cregContainer

	// Extract information about env for TestContext
	consulPort, err := consulContainer.MappedPort(ctx, "8500")
	if err != nil {
		return TestContext{}, fmt.Errorf("failed to get mapped port: %w", err)
	}

	consulName, err := consulContainer.Name(ctx)
	if err != nil {
		return TestContext{}, fmt.Errorf("failed to get container name: %w", err)
	}
	cregContainerName, err := cregContainer.Name(ctx)
	if err != nil {
		return TestContext{}, fmt.Errorf("failed to get container name: %w", err)
	}

	return TestContext{
		CregContainerName:   cregContainerName,
		ConsulContainerName: consulName,
		ConsulPort:          consulPort.Port(),
		Network:             newNetwork.Name,
	}, nil
}

func (tc *TestCase) Teardown() {
	ctx := context.Background()
	if tc.cregContainer != nil {
		_ = tc.cregContainer.Terminate(ctx)
	}
	if tc.consulContainer != nil {
		_ = tc.consulContainer.Terminate(ctx)
	}
	// Remove the Docker network
	_ = exec.Command("docker", "network", "rm", tc.network.Name).Run()
}

func (tc *TestCase) Execute(t *testing.T) {
	tc.Name = t.Name()

	tCtx, err := tc.Setup()
	if err != nil {
		t.Fatalf("Failed to setup test case '%s': %s", tc.Name, err)
	}
	// defer tc.Teardown()

	// time.Sleep(2 * time.Second)

	tc.Run(t, &tCtx)

	if t.Failed() {
		cmd := exec.Command("docker", "logs", tCtx.CregContainerName)
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("Could not get creg logs: %s", err)
		} else {
			log.Printf("Logs from creg:\n%s", out)
		}
	}
}

func IsConsulServiceRegistered(serviceName, port string) (bool, error) {
	resp, err := http.Get("http://localhost:" + port + "/v1/catalog/service/" + serviceName)
	if err != nil {
		return false, errors.Wrapf(err, "could not get consul service %s", serviceName)
	}
	defer resp.Body.Close()

	var services []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&services)
	if err != nil {
		return false, errors.Wrap(err, "could not decode body")
	}

	return len(services) > 0, nil
}
