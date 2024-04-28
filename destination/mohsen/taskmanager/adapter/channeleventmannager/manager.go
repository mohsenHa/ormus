package channeleventmannager

import (
	"github.com/ormushq/ormus/destination/mohsen/eventmanager"
	"github.com/ormushq/ormus/destination/mohsen/taskmanager"
	"github.com/ormushq/ormus/destination/taskservice"
	"sync"
)

type ChannelEventManager struct {
	taskChannelConsumer  taskmanager.TaskChannelConsumer
	taskIdempotency      taskservice.Idempotency
	eventChannelConsumer eventmanager.EventChannelConsumer
}

func New(taskIdempotency taskservice.Idempotency, eventChannelConsumer eventmanager.EventChannelConsumer) (ChannelEventManager, error) {
	return ChannelEventManager{
		taskIdempotency:      taskIdempotency,
		eventChannelConsumer: eventChannelConsumer,
	}, nil
}

func (c ChannelEventManager) GetTaskChannelConsumer(done <-chan bool, wg *sync.WaitGroup) (taskmanager.TaskChannelConsumer, error) {
	return c.taskChannelConsumer, nil
}
