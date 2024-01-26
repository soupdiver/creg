package integratontest

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/pkg/errors"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type TestCase struct {
	Name     string
	CregArgs []string
	RunFunc  func(t *testing.T, ctx *TestContext)

	UseConsul bool
	UseEtcd   bool

	network         *testcontainers.DockerNetwork
	cregContainer   testcontainers.Container
	consulContainer testcontainers.Container
}

type TestContext struct {
	CregID            string
	CregContainerName string
	Network           string

	ConsulContainerName string
	ConsulPort          string

	EtcdContainerName string
	EtcdPort          string
}

func (tc *TestCase) Setup() (TestContext, error) {
	ctx := context.Background()

	tCtx := TestContext{}

	newNetwork, err := network.New(ctx)
	if err != nil {
		return TestContext{}, fmt.Errorf("failed to create docker network: %w", err)
	}
	tc.network = newNetwork

	// generate random 12 char string
	rand.Seed(time.Now().UnixNano())
	letterRunes := []rune("abcdefghijklmnopqrstuvwxyz")
	b := make([]rune, 12)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	tCtx.CregID = string(b)

	// Start Consul container if needed
	if tc.UseConsul {
		consulContainer, err := StartConsulContainer(ctx, newNetwork)
		if err != nil {
			return TestContext{}, fmt.Errorf("failed to start consul container: %w", err)
		}
		tc.consulContainer = consulContainer

		consulContainerIP, err := tc.consulContainer.ContainerIP(ctx)
		if err != nil {
			return TestContext{}, fmt.Errorf("failed to get container IP: %w", err)
		}
		consulContainerIP = strings.Trim(consulContainerIP, "/")

		for i, arg := range tc.CregArgs {
			tc.CregArgs[i] = strings.Replace(arg, "CONSUL_CONTAINER", consulContainerIP, -1)
		}

		tCtx.ConsulContainerName, err = tc.consulContainer.Name(ctx)
		if err != nil {
			return TestContext{}, fmt.Errorf("failed to get container name: %w", err)
		}

		consulPort, err := tc.consulContainer.MappedPort(ctx, "8500")
		if err != nil {
			return TestContext{}, fmt.Errorf("failed to get mapped port: %w", err)
		}

		tCtx.ConsulPort = consulPort.Port()
	}

	if tc.UseEtcd {
		// Start Etcd container
		etcdContainer, err := StartEtcdContainer(ctx, newNetwork)
		if err != nil {
			return TestContext{}, fmt.Errorf("failed to start etcd container: %w", err)
		}

		etcdContainerIP, err := etcdContainer.ContainerIP(ctx)
		if err != nil {
			return TestContext{}, fmt.Errorf("failed to get container IP: %w", err)
		}

		for i, arg := range tc.CregArgs {
			tc.CregArgs[i] = strings.Replace(arg, "ETCD_ADDRESS", etcdContainerIP, -1)
		}

		tCtx.EtcdContainerName, err = etcdContainer.Name(ctx)
		if err != nil {
			return TestContext{}, fmt.Errorf("failed to get container name: %w", err)
		}

		etcdPort, err := etcdContainer.MappedPort(ctx, "2379")
		if err != nil {
			return TestContext{}, fmt.Errorf("failed to get mapped port: %w", err)
		}

		tCtx.EtcdPort = etcdPort.Port()
	}

	// Start Creg container with provided arguments
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
		Cmd: append([]string{"/creg", "--id", tCtx.CregID}, tc.CregArgs...),
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
	cregContainerName, err := cregContainer.Name(ctx)
	if err != nil {
		return TestContext{}, fmt.Errorf("failed to get container name: %w", err)
	}

	tCtx.CregContainerName = cregContainerName
	tCtx.Network = newNetwork.Name

	return tCtx, nil
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

	tc.RunFunc(t, &tCtx)

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

func IsEtcdServiceRegistered(serviceName, port, id string) (bool, error) {
	EtcdClient, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"http://localhost:" + port},
		DialTimeout: 5 * time.Second,
	})

	if err != nil {
		return false, errors.Wrapf(err, "could not create etcd client")
	}

	// log.Printf("check prefix: %s", id)

	s, err := EtcdClient.Get(context.Background(), "creg/"+id, clientv3.WithPrefix())
	if err != nil {
		return false, errors.Wrapf(err, "could not get etcd client")
	}
	// log.Printf("res2: %+v", s)

	return s.Count > 0, nil

	// log.Printf("Checking via: " + "http://localhost:" + port + "/v3/kv/" + serviceName + "?keys=true")
	// resp, err := http.Get("http://localhost:" + port + "/v3/kv/" + serviceName + "?keys=true")
	// if err != nil {
	// 	return false, errors.Wrapf(err, "could not get etcd service %s", serviceName)
	// }
	// defer resp.Body.Close()

	// // print body
	// body, err := ioutil.ReadAll(resp.Body)
	// if err != nil {
	// 	return false, errors.Wrap(err, "could not read body")
	// }
	// fmt.Println(string(body))

	// if resp.StatusCode != http.StatusOK {
	// 	return false, nil
	// }

	// var services []map[string]interface{}
	// err = json.NewDecoder(resp.Body).Decode(&services)
	// if err != nil {
	// 	return false, errors.Wrap(err, "could not decode body")
	// }

	// return len(services) > 0, nil
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

func StartEtcdContainer(ctx context.Context, network *testcontainers.DockerNetwork) (testcontainers.Container, error) {
	etcdReq := testcontainers.ContainerRequest{
		// Name:         "etcd",
		Image:        "bitnami/etcd:3.5",
		ExposedPorts: []string{"2379/tcp"},
		WaitingFor:   wait.ForListeningPort("2379/tcp"),
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.NetworkMode = container.NetworkMode(network.ID)
		},
		Env: map[string]string{
			"ALLOW_NONE_AUTHENTICATION": "yes",
		},
	}
	etcdContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: etcdReq,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start etcd container: %w", err)
	}

	return etcdContainer, nil
}

func StartConsulContainer(ctx context.Context, network *testcontainers.DockerNetwork) (testcontainers.Container, error) {
	consulReq := testcontainers.ContainerRequest{
		// Name:         "consull",
		Image:        "consul:1.15",
		ExposedPorts: []string{"8500/tcp"},
		WaitingFor:   wait.ForListeningPort("8500/tcp"),
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.NetworkMode = container.NetworkMode(network.ID)
		},
	}
	consulContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: consulReq,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start consul container: %w", err)
	}

	return consulContainer, nil
}
