package channeltaskmanager

import (
	"fmt"
	"github.com/ormushq/ormus/destination/entity/taskentity"
	tasktype "github.com/ormushq/ormus/destination/newimplementation/task"
	"sync"
)

type Manager struct {
	taskChannels map[tasktype.TaskType]chan taskentity.Task
	wg           *sync.WaitGroup
	done         <-chan bool
}

func New(done <-chan bool, wg *sync.WaitGroup) (*Manager, error) {
	return &Manager{
		wg:           wg,
		done:         done,
		taskChannels: make(map[tasktype.TaskType]chan taskentity.Task),
	}, nil
}

func (m *Manager) NewChannel(taskType tasktype.TaskType, bufferSize int) {
	m.taskChannels[taskType] = make(chan taskentity.Task, bufferSize)
}

func (m *Manager) GetTaskChannelForConsume(taskType tasktype.TaskType) (chan taskentity.Task, error) {
	channel, ok := m.taskChannels[taskType]
	if !ok {
		return nil, fmt.Errorf("task channel not found: %v", taskType)
	}
	return channel, nil
}

func (m *Manager) GetTaskChannelForPublish(taskType tasktype.TaskType) (chan taskentity.Task, error) {
	return m.GetTaskChannelForConsume(taskType)
}
