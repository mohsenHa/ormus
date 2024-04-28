package taskmanager

import (
	"github.com/ormushq/ormus/destination/entity/taskentity"
	"sync"
)

type TaskManager interface {
	GetTaskChannelConsumer(done <-chan bool, wg *sync.WaitGroup) (TaskChannelConsumer, error)
}

type TaskChannelConsumer *<-chan taskentity.Task
