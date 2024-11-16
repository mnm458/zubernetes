package main

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"
	"github.com/mnm458/zubernetes/task"
	"github.com/mnm458/zubernetes/worker"
)

func createContainer() (*task.Docker, *task.DockerResult) {
	c := task.Config{
		Name:  "test-container-1",
		Image: "postgres:13",
		Env: []string{
			"POSTGRES_USER=cube",
			"POSTGRES_PASSWORD=secret",
		},
	}
	dc, _ := client.NewClientWithOpts(client.FromEnv)
	d := task.Docker{
		Client: dc,
		Config: c,
	}
	result := d.Run()
	if result.Error != nil {
		fmt.Println(result.Error)
		return nil, nil
	}
	d.ContainerId = result.ContainerId
	fmt.Printf("Container %s is running with config %v\n", result.ContainerId, d.Config)
	return &d, &result
}

func stopContainer(d *task.Docker) *task.DockerResult {
	ctx := context.Background()

	// Check if the container exists
	fmt.Println("stopping container with this ID: ", d.ContainerId)
	_, err := d.Client.ContainerInspect(ctx, d.ContainerId)
	if err != nil {
		if client.IsErrNotFound(err) {
			fmt.Printf("Container %s not found. It may have already been removed.\n", d.ContainerId)
			fmt.Println("got error: ", err)
			return &task.DockerResult{
				Error:       err,
				Action:      "stop",
				ContainerId: d.ContainerId,
				Result:      "container not found",
			}
		}
		fmt.Printf("Error inspecting container %s: %v\n", d.ContainerId, err)
		return &task.DockerResult{Error: err, Action: "stop", ContainerId: d.ContainerId}
	}

	// Attempt to stop the container
	timeout := 30
	err = d.Client.ContainerStop(ctx, d.ContainerId, container.StopOptions{Timeout: &timeout})
	if err != nil {
		fmt.Printf("Error stopping container %s: %v\n", d.ContainerId, err)
		return &task.DockerResult{Error: err, Action: "stop", ContainerId: d.ContainerId}
	}

	// Attempt to remove the container
	err = d.Client.ContainerRemove(ctx, d.ContainerId, container.RemoveOptions{})
	if err != nil {
		fmt.Printf("Error removing container %s: %v\n", d.ContainerId, err)
		return &task.DockerResult{Error: err, Action: "remove", ContainerId: d.ContainerId}
	}

	fmt.Printf("Container %s has been stopped and removed\n", d.ContainerId)
	return &task.DockerResult{
		Action:      "stop and remove",
		ContainerId: d.ContainerId,
		Result:      "success",
	}
}
func main() {
	// t := task.Task{
	// 	ID:     uuid.New(),
	// 	Name:   "Task-1",
	// 	State:  task.Pending,
	// 	Image:  "Image-1",
	// 	Memory: 1024,
	// 	Disk:   1,
	// }
	// te := task.TaskEvent{
	// 	ID:        uuid.New(),
	// 	State:     task.Pending,
	// 	Timestamp: time.Now(),
	// 	Task:      t,
	// }
	// fmt.Printf("task: %v\n", t)
	// fmt.Printf("task event: %v\n", te)

	// w := worker.Worker{
	// 	Queue: *queue.New(),
	// 	Db:    make(map[uuid.UUID]task.Task),
	// }
	// fmt.Printf("worker: %v\n", w)
	// w.CollectStats()
	// w.RunTask()
	// w.StartTask()
	// w.StopTask()
	// m := manager.Manager{
	// 	Pending: *queue.New(),
	// 	TaskDb:  make(map[string][]task.Task),
	// 	EventDb: make(map[string][]task.TaskEvent),
	// 	Workers: []string{w.Name},
	// }
	// fmt.Printf("manager: %v\n", m)
	// m.SelectWorker()
	// m.UpdateTasks()
	// m.SendWork()

	// n := node.Node{
	// 	Name:   "Name-1",
	// 	Ip:     "192.168.1.1",
	// 	Cores:  4,
	// 	Memory: 1024,
	// 	Disk:   25,
	// 	Role:   "worker",
	// }
	// fmt.Printf("node: %v\n", n)

	// fmt.Println("create a test container")
	// dockerTask, createResult := createContainer()
	// if createResult.Error != nil {
	// 	fmt.Println(createResult.Error)
	// 	os.Exit(1)
	// }
	// time.Sleep(time.Second * 5)
	// fmt.Printf("stopping container %s\n", createResult.ContainerId)
	// _ = stopContainer(dockerTask)
	db := make(map[uuid.UUID]*task.Task)
	w := worker.Worker{
		Queue: *queue.New(),
		Db:    db,
	}
	t := task.Task{
		ID:    uuid.New(),
		Name:  "test-container-1",
		State: task.Scheduled,
		Image: "strm/helloworld-http",
	}
	// first time the worker will see the task
	fmt.Println("starting task")
	w.AddTask(t)
	result := w.RunTask()
	if result.Error != nil {
		panic(result.Error)
	}
	t.ContainerID = result.ContainerId
	fmt.Printf("task %s is running in container %s\n", t.ID, t.ContainerID)
	fmt.Println("Sleepy time")
	time.Sleep(time.Second * 30)
	fmt.Printf("stopping task %s\n", t.ID)
	t.State = task.Completed
	w.AddTask(t)
	result = w.RunTask()
	if result.Error != nil {
		panic(result.Error)
	}
}
