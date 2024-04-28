package workermanager

import (
	"github.com/ormushq/ormus/destination/mohsen/taskmanager"
	"github.com/ormushq/ormus/destination/mohsen/worker"
	"github.com/ormushq/ormus/event"
)

type WorkerManager struct {
	workers               map[string]worker.Worker
	taskChannelConsumer   taskmanager.TaskChannelConsumer
	eventChannelPublisher *chan<- event.ProcessedEvent
}

func New(taskChannelConsumer taskmanager.TaskChannelConsumer, eventChannelPublisher *chan<- event.ProcessedEvent) (WorkerManager, error) {
	return WorkerManager{
		taskChannelConsumer:   taskChannelConsumer,
		eventChannelPublisher: eventChannelPublisher,
	}, nil
}
func (w WorkerManager) RegisterWorker(workerType string, worker worker.Worker) error {
	w.workers[workerType] = worker
	return nil
}
