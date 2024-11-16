package worker

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"
	"github.com/mnm458/zubernetes/task"
)

type Worker struct {
	Name             string
	Queue            queue.Queue
	Db               map[uuid.UUID]*task.Task
	TaskCount        int
	CadvisorEndpoint string
}

func (w *Worker) AddTask(t task.Task) {
	w.Queue.Enqueue(t)
}

func (w *Worker) StartTask(t task.Task) task.DockerResult {
	t.StartTime = time.Now().UTC()
	config := task.NewConfig(&t)
	d, err := task.NewDocker(config)
	if err != nil {
		return task.DockerResult{Error: err}
	}
	res := d.Run()
	if res.Error != nil {
		log.Printf("Error running task %v: %v\n", t.ID, res.ContainerId)
		t.State = task.Failed
		w.Db[t.ID] = &t
		return res
	}
	t.ContainerID = res.ContainerId
	t.State = task.Running
	w.Db[t.ID] = &t
	return res
}

func (w *Worker) RunTask() task.DockerResult {
	t := w.Queue.Dequeue()
	if t == nil {
		log.Println("No tasks in the queue")
		return task.DockerResult{}
	}
	taskQueued := t.(task.Task)
	taskPersisted := w.Db[taskQueued.ID]
	if taskPersisted == nil {
		taskPersisted = &taskQueued
		w.Db[taskQueued.ID] = &taskQueued
	}
	var result task.DockerResult
	if task.ValidStateTransition(taskPersisted.State, taskQueued.State) {
		switch taskQueued.State {
		case task.Scheduled:
			result = w.StartTask(taskQueued)
		case task.Completed:
			result = w.StopTask(taskQueued)
		default:
			result.Error = errors.New("invalid transition state")
		}
	} else {
		err := fmt.Errorf("Invalid transition from %v to %v\n", taskPersisted.State, taskQueued.State)
		result.Error = err
	}
	return result
}

func (w *Worker) StopTask(t task.Task) task.DockerResult {
	config := task.NewConfig(&t)
	d, err := task.NewDocker(config)
	if err != nil {
		return task.DockerResult{Error: err}
	}
	res := d.Stop(t.ContainerID)
	if res.Error != nil {
		log.Printf("Error stopping container %v: %v", t.ContainerID, res.Error)
	}
	t.FinishTime = time.Now().UTC()
	t.State = task.Completed
	w.Db[t.ID] = &t
	log.Printf("Stopped and removed container %v for task %v", t.ContainerID, t.ID)
	return res
}

func (w *Worker) CollectStats() {
	fmt.Println("CollectStats called")

}

func (w *Worker) GetTasks() []*task.Task {
	res := make([]*task.Task, len(w.Db))
	i := 0
	for _, t := range w.Db {
		res[i] = t
		i++
	}
	return res
}

func NewWorker(name string, cadvisorEndpoint string) *Worker {

	return &Worker{
		Queue:            *queue.New(),
		Db:               make(map[uuid.UUID]*task.Task),
		CadvisorEndpoint: cadvisorEndpoint,
	}
}
