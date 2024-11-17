package task

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
)

type State int

const (
	Pending State = iota
	Scheduled
	Running
	Completed
	Failed
)

const (
	StandardBinaryName = "app.bin"
	StandardBuildName  = "build"
	GSCScriptName      = "launch-gsc.sh"
)

type Task struct {
	ID            uuid.UUID         `protobuf:"bytes,1,opt,name=id"`
	ContainerID   string            `protobuf:"bytes,2,opt,name=container_id,json=containerId"`
	Name          string            `protobuf:"bytes,3,opt,name=name"`
	State         State             `protobuf:"varint,4,opt,name=state"`
	Image         string            `protobuf:"bytes,5,opt,name=image"`
	Memory        int               `protobuf:"varint,6,opt,name=memory"`
	Disk          int               `protobuf:"varint,7,opt,name=disk"`
	ExposedPorts  nat.PortSet       `protobuf:"bytes,8,rep,name=exposed_ports,json=exposedPorts"`
	PortBindings  map[string]string `protobuf:"bytes,9,rep,name=port_bindings,json=portBindings"`
	RestartPolicy string            `protobuf:"bytes,10,opt,name=restart_policy,json=restartPolicy"`
	StartTime     time.Time         `protobuf:"bytes,11,opt,name=start_time,json=startTime"`
	FinishTime    time.Time         `protobuf:"bytes,12,opt,name=finish_time,json=finishTime"`
}

type TaskEvent struct {
	ID        uuid.UUID `protobuf:"bytes,1,opt,name=id"`
	State     State     `protobuf:"varint,2,opt,name=state"`
	Timestamp time.Time `protobuf:"bytes,3,opt,name=timestamp"`
	Task      Task      `protobuf:"bytes,4,opt,name=task"`
}

type Config struct {
	Name          string
	AttachStdin   bool
	AttachStdout  bool
	AttachStderr  bool
	Cmd           []string
	Image         string
	Memory        int64
	Disk          int64
	Env           []string
	RestartPolicy string
	Runtime       Runtime
	GSCConfig     *GSCConfig `protobuf:"bytes,13,opt,name=gsc_config,json=gscConfig"`
	IsGSC         bool       `protobuf:"varint,14,opt,name=is_gsc,json=isGsc"`
}
type Runtime struct {
	ContainerID string
}
type Docker struct {
	Client      *client.Client
	Config      Config
	ContainerId string
}

type DockerResult struct {
	Error       error
	Action      string
	ContainerId string
	Result      string
}
type GSCConfig struct {
	SGXDevicePath  string // Path to SGX device (e.g., "/dev/sgx_enclave")
	AESMSocketPath string // Path to AESM socket
	BinaryInput    string // Path to input binary file
	RustBuildInput string // Path to input Rust build
	ScriptPath     string // Path to GSC launch script
}

var stateTransitionMap = map[State][]State{
	Pending:   []State{Scheduled},
	Scheduled: []State{Scheduled, Running, Failed},
	Running:   []State{Running, Completed, Failed},
	Completed: []State{},
	Failed:    []State{},
}

func Contains(states []State, state State) bool {
	for _, s := range states {
		if s == state {
			return true
		}
	}
	return false
}

func ValidStateTransition(src State, dst State) bool {
	return Contains(stateTransitionMap[src], dst)
}

func (d *Docker) Run() DockerResult {
	ctx := context.Background()
	reader, err := d.Client.ImagePull(ctx, d.Config.Image, image.PullOptions{})
	if err != nil {
		log.Printf("Error pulling image %s: %v\n", d.Config.Image, err)
		return DockerResult{Error: err}
	}
	io.Copy(os.Stdout, reader)
	rp := container.RestartPolicy{
		Name: container.RestartPolicyMode(d.Config.RestartPolicy),
	}
	r := container.Resources{
		Memory: d.Config.Memory,
	}
	cc := container.Config{
		Image: d.Config.Image,
		Env:   d.Config.Env,
	}
	hc := container.HostConfig{
		RestartPolicy:   rp,
		Resources:       r,
		PublishAllPorts: true,
	}
	resp, err := d.Client.ContainerCreate(ctx, &cc, &hc, nil, nil, d.Config.Name)
	if err != nil {
		log.Printf("Error creating container using image %s: %v\n", d.Config.Image, err)
		return DockerResult{Error: err}
	}
	err = d.Client.ContainerStart(ctx, resp.ID, container.StartOptions{})
	if err != nil {
		log.Printf("Error starting container %s: %v\n", resp.ID, err)
		return DockerResult{Error: err}
	}
	d.Config.Runtime.ContainerID = resp.ID
	out, err := d.Client.ContainerLogs(ctx, resp.ID, container.LogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		log.Printf("Error getting logs for container %s: %v\n", resp.ID, err)
		return DockerResult{Error: err}
	}
	stdcopy.StdCopy(os.Stdout, os.Stderr, out)
	return DockerResult{
		ContainerId: resp.ID,
		Action:      "start",
		Result:      "success",
	}
}

func (d *Docker) RunGcs() DockerResult {
	ctx := context.Background()

	if d.Config.IsGSC {
		// Prepare workspace for GSC
		workDir, err := d.prepareGSCWorkspace()
		if err != nil {
			return DockerResult{Error: err}
		}
		defer os.RemoveAll(workDir) // Cleanup after container is started

		// Create container with GSC configuration
		return d.runGSCContainer(ctx, workDir)
	}

	// Regular container logic...
	return d.Run()
}

// Prepare workspace for GSC
func (d *Docker) prepareGSCWorkspace() (string, error) {
	// Create temporary workspace
	workDir, err := os.MkdirTemp("", "gsc-workspace-")
	if err != nil {
		return "", fmt.Errorf("failed to create workspace: %v", err)
	}

	// Copy binary with standardized name
	if err := copyFile(d.Config.GSCConfig.BinaryInput,
		filepath.Join(workDir, StandardBinaryName)); err != nil {
		os.RemoveAll(workDir)
		return "", fmt.Errorf("failed to copy binary: %v", err)
	}

	// Copy Rust build with standardized name
	if err := copyFile(d.Config.GSCConfig.RustBuildInput,
		filepath.Join(workDir, StandardBuildName)); err != nil {
		os.RemoveAll(workDir)
		return "", fmt.Errorf("failed to copy build: %v", err)
	}

	// Copy GSC launch script
	if err := copyFile(d.Config.GSCConfig.ScriptPath,
		filepath.Join(workDir, GSCScriptName)); err != nil {
		os.RemoveAll(workDir)
		return "", fmt.Errorf("failed to copy launch script: %v", err)
	}

	return workDir, nil
}

func (d *Docker) runGSCContainer(ctx context.Context, workDir string) DockerResult {
	// Container configuration
	cc := &container.Config{
		Image: d.Config.Image,
		Cmd:   []string{"/workspace/" + GSCScriptName}, // Run the launch script
		Env:   d.Config.Env,
	}

	// Host configuration
	hc := &container.HostConfig{
		Binds: []string{
			// Mount workspace directory
			fmt.Sprintf("%s:/workspace", workDir),
			// Mount SGX device and AESM socket
			fmt.Sprintf("%s:%s", d.Config.GSCConfig.SGXDevicePath, "/dev/sgx_enclave"),
			fmt.Sprintf("%s:%s", d.Config.GSCConfig.AESMSocketPath, "/var/run/aesmd/aesm.socket"),
		},
		// Add SGX device
		// Devices: []container.DeviceMapping{
		// 	{
		// 		PathOnHost:        d.Config.GSCConfig.SGXDevicePath,
		// 		PathInContainer:   "/dev/sgx_enclave",
		// 		CgroupPermissions: "rwm",
		// 	},
		// },
		RestartPolicy: container.RestartPolicy{
			Name: container.RestartPolicyMode(d.Config.RestartPolicy),
		},
		Resources: container.Resources{
			Memory: d.Config.Memory,
		},
	}

	// Create and start container
	resp, err := d.Client.ContainerCreate(ctx, cc, hc, nil, nil, d.Config.Name)
	if err != nil {
		return DockerResult{Error: err}
	}

	// Start container
	if err := d.Client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return DockerResult{Error: err}
	}

	// Get logs
	out, err := d.Client.ContainerLogs(ctx, resp.ID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		return DockerResult{Error: err}
	}
	stdcopy.StdCopy(os.Stdout, os.Stderr, out)

	return DockerResult{
		ContainerId: resp.ID,
		Action:      "start",
		Result:      "success",
	}
}

// Helper function to copy files
func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}

func (d *Docker) Stop(id string) DockerResult {
	log.Printf("Attempting to stop container %v", id)
	ctx := context.Background()
	err := d.Client.ContainerStop(ctx, id, container.StopOptions{})
	if err != nil {
		log.Printf("Error stoping container: %v %v\n", id, err)
		panic(err)
	}
	err = d.Client.ContainerRemove(ctx, id, container.RemoveOptions{})
	if err != nil {
		panic(err)
	}
	return DockerResult{Action: "stop", Result: "success", Error: nil}
}

// ID            uuid.UUID
//
//	Name          string
//	State         State
//	Image         string
//	Memory        int
//	Disk          int
//	ExposedPorts  nat.PortSet
//	PortBindings  map[string]string
//	RestartPolicy string
//	StartTime     time.Time
//	FinishTime    time.Time
func NewConfig(t *Task) Config {
	return Config{
		Name:         t.Name,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Image:        t.Image,
		Memory:       int64(t.Memory),
		// AttachStdin   bool
		// AttachStdout  bool
		// AttachStderr  bool
		// Cmd           []string
		// Image         string
		// Memory        int64
		// Disk          int64
		// Env           []string
		// RestartPolicy string
		// Runtime       Runtime
	}
}

func NewDocker(c Config) (*Docker, error) {
	newClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, err
	}
	return &Docker{
		Client: newClient,
		Config: c,
	}, nil
}
