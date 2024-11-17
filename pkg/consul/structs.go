package integration

import "github.com/hashicorp/consul/connect"

// This would need more types for production-level use
// Setting the minimum here to get started.
type ServiceHealthCheckConfig struct {
	GRPC     string
	Interval string
}

// This would need more types for production-level use
// Setting the minimum here to get started.
type ServiceConfig struct {
	Name  string
	Port  int
	Check ServiceHealthCheckConfig
}

type Service struct {
	ConsulService *connect.Service
	Config        *ServiceConfig
}
