package integratontest_test

import (
	"context"
	"io"
	"log"
	"os"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	it "github.com/soupdiver/creg/integration_test"
)

func TestConsulServiceRegistrationWithoutLabels(t *testing.T) {
	t.Parallel()
	tc := it.TestCase{
		CregArgs: []string{
			"--address", "6.6.6.6",
			"--consul", "http://CONSUL_CONTAINER:8500",
		},
		UseConsul: true,
		RunFunc: func(t *testing.T, tCtx *it.TestContext) {
			ctx := context.Background()
			serviceName := t.Name()

			myservice, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
				ContainerRequest: testcontainers.ContainerRequest{
					Name:       serviceName,
					Image:      "nginx",
					WaitingFor: wait.ForHTTP("/"),
					Networks:   []string{tCtx.Network},
				},
				Started: true,
			})
			if err != nil {
				t.Fatalf("Failed to start container: %s", err)
			}
			defer myservice.Terminate(ctx)

			registered, err := it.IsConsulServiceRegistered(serviceName, tCtx.ConsulPort)
			if err != nil {
				t.Fatalf("Failed to check if service is registered: %s", err)
			}

			if registered {
				t.Errorf("Expected service to not be registered, but it was")
			}
		},
	}

	tc.Execute(t)
}

func TestConsulServiceRegistrationWithWrongCregId(t *testing.T) {
	t.Parallel()
	tc := it.TestCase{
		CregArgs: []string{
			"--address", "6.6.6.6",
			"--consul", "http://CONSUL_CONTAINER:8500",
		},
		UseConsul: true,
		RunFunc: func(t *testing.T, tCtx *it.TestContext) {
			ctx := context.Background()
			serviceName := t.Name()

			myservice, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
				ContainerRequest: testcontainers.ContainerRequest{
					Name:  serviceName,
					Image: "nginx",
					Labels: map[string]string{
						"creg":          "true",
						"creg.port":     "'80/tcp:" + serviceName + "'",
						"creg.backends": "consul",
						"creg.id":       "foo",
					},
					WaitingFor: wait.ForHTTP("/"),
					Networks:   []string{tCtx.Network},
				},
				Started: true,
			})
			if err != nil {
				t.Fatalf("Failed to start container: %s", err)
			}
			defer myservice.Terminate(ctx)

			registered, err := it.IsConsulServiceRegistered(serviceName, tCtx.ConsulPort)
			if err != nil {
				t.Fatalf("Failed to check if service is registered: %s", err)
			}

			if registered {
				t.Errorf("Expected service to not be registered, but it was")
			}
		},
	}

	tc.Execute(t)
}

func TestConsulServiceRegistrationWithLabels(t *testing.T) {
	t.Parallel()
	tc := it.TestCase{
		CregArgs: []string{
			"--address", "6.6.6.6",
			"--consul", "http://CONSUL_CONTAINER:8500",
		},
		UseConsul: true,
		RunFunc: func(t *testing.T, tCtx *it.TestContext) {
			ctx := context.Background()
			serviceName := t.Name()

			myservice, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
				ContainerRequest: testcontainers.ContainerRequest{
					Name:  serviceName,
					Image: "nginx",
					Labels: map[string]string{
						"creg":          "true",
						"creg.port":     "'80/tcp:" + serviceName + "'",
						"creg.backends": "consul",
						"creg.id":       tCtx.CregID,
					},
					ExposedPorts: []string{"80/tcp"},
					WaitingFor:   wait.ForHTTP("/"),
				},
				Started: true,
			})
			if err != nil {
				t.Fatalf("Failed to start container: %s", err)
			}

			registered, err := it.IsConsulServiceRegistered(serviceName, tCtx.ConsulPort)
			if err != nil {
				t.Fatalf("Failed to check if service is registered: %s", err)
			}

			if !registered {
				t.Errorf("Expected service %s to be registered, but it was not", serviceName)
			}

			// Terminate and check if service is unregistered
			err = myservice.Terminate(ctx)
			if err != nil {
				t.Fatalf("Failed to terminate container: %s", err)
			}

			registered, err = it.IsConsulServiceRegistered(serviceName, tCtx.ConsulPort)
			if err != nil {
				t.Fatalf("Failed to check if service is registered: %s", err)
			}

			if registered {
				t.Errorf("Expected service %s to not be registered, but it was", serviceName)
			}
		},
	}

	tc.Execute(t)
}

func TestEtcdServiceRegistrationWithoutLabels(t *testing.T) {
	t.Parallel()
	tc := it.TestCase{
		CregArgs: []string{
			"--address", "6.6.6.6",
			"--etcd", "http://ETCD_ADDRESS:2379",
		},
		UseEtcd: true,
		RunFunc: func(t *testing.T, tCtx *it.TestContext) {
			ctx := context.Background()
			serviceName := t.Name()

			myservice, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
				ContainerRequest: testcontainers.ContainerRequest{
					Name:       serviceName,
					Image:      "nginx",
					WaitingFor: wait.ForHTTP("/"),
					Networks:   []string{tCtx.Network},
				},
				Started: true,
			})
			if err != nil {
				t.Fatalf("Failed to start container: %s", err)
			}
			defer myservice.Terminate(ctx)

			registered, err := it.IsEtcdServiceRegistered("creg/"+serviceName, tCtx.EtcdPort, tCtx.CregID)
			if err != nil {
				t.Fatalf("Failed to check if service is registered: %s", err)
			}

			if registered {
				t.Errorf("Expected service to not be registered, but it was")
			}
		},
	}

	tc.Execute(t)
}

func TestEtcdServiceRegistrationWitLabels(t *testing.T) {
	t.Parallel()
	tc := it.TestCase{
		CregArgs: []string{
			"--address", "6.6.6.6",
			"--etcd", "http://ETCD_ADDRESS:2379",
		},
		UseEtcd: true,
		RunFunc: func(t *testing.T, tCtx *it.TestContext) {
			ctx := context.Background()
			serviceName := t.Name()

			myservice, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
				ContainerRequest: testcontainers.ContainerRequest{
					Name:  serviceName,
					Image: "nginx",
					Labels: map[string]string{
						"creg":          "true",
						"creg.port":     "'80/tcp:" + serviceName + "'",
						"creg.backends": "etcd",
						"creg.id":       tCtx.CregID,
					},
					WaitingFor: wait.ForHTTP("/"),
					Networks:   []string{tCtx.Network},
				},
				Started: true,
			})
			if err != nil {
				t.Fatalf("Failed to start container: %s", err)
			}
			defer myservice.Terminate(ctx)

			registered, err := it.IsEtcdServiceRegistered("creg", tCtx.EtcdPort, tCtx.CregID)
			if err != nil {
				t.Fatalf("Failed to check if service is registered: %s", err)
			}

			if !registered {
				t.Errorf("Expected service to be registered, but it was not")
			}
		},
	}

	tc.Execute(t)
}

func TestMain(m *testing.M) {
	// Disable logging for testcontainers
	testcontainers.Logger = log.New(io.Discard, "", log.LstdFlags)

	// Run tests
	code := m.Run()

	os.Exit(code)
}
