package integration

import (
	"log"

	capi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/connect"
)

type MeshClient struct {
	ConsulClient *capi.Client
}

// Must be run as part of the worker registration process
func (m *MeshClient) SetupClient() {
	// Create a Consul client
	client, err := capi.NewClient(capi.DefaultConfig())
	if err != nil {
		log.Fatalf("Failed to create Consul client: %v", err)
	}

	m.ConsulClient = client
}

// Must be run before the task is started on a worker node
func (m *MeshClient) RegisterTask(config *ServiceConfig) Service {
	// Create a Consul service
	service := &capi.AgentServiceRegistration{
		Name: config.Name,
		Port: config.Port,
		Check: &capi.AgentServiceCheck{
			GRPC:     config.Check.Interval,
			Interval: config.Check.Interval,
		},
	}
	if err := m.ConsulClient.Agent().ServiceRegister(service); err != nil {
		log.Fatalf("Failed to register service: %v", err)
	}

	// Create a Connect-enabled service
	svc, err := connect.NewService(config.Name, m.ConsulClient)
	if err != nil {
		log.Fatalf("Failed to create Connect service: %v", err)
	}

	return Service{
		ConsulService: svc,
		Config:        config,
	}
}

// Must be run after a task is finished.
// Recommended to run in a deferred manner
func (m *MeshClient) DeregisterTask(service *Service) {
	m.ConsulClient.Agent().ServiceDeregister(service.Config.Name)
	service.ConsulService.Close()
}
