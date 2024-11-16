package manager

import (
	"fmt"

	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"
	. "github.com/mnm458/zubernetes/task"
)

type Manager struct {
	Pending       queue.Queue
	TaskDb        map[string][]Task
	EventDb       map[string][]TaskEvent
	Workers       []string
	WorkerTaskMap map[string][]uuid.UUID
	TaskWorkerMap map[uuid.UUID]string
}

func (m *Manager) SelectWorker() {
	fmt.Println("SelectWorker called")
}

func (m *Manager) UpdateTasks() {
	fmt.Println("UpdateTasks called")
}

func (m *Manager) SendWork() {
	fmt.Println("SendWork called")

}

func (m *Manager) ProvisonWorkerNode() {
	// -> calls the setup_worker_node
}
