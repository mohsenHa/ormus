package taskmanager

import (
	"context"
	"fmt"
	"github.com/ormushq/ormus/destination/entity/taskentity"
	"github.com/ormushq/ormus/destination/newimplementation/eventmanager"
	tasktype "github.com/ormushq/ormus/destination/newimplementation/task"
	"github.com/ormushq/ormus/destination/taskservice"
	event2 "github.com/ormushq/ormus/event"
	"log/slog"
	"sync"
)

type Adapter interface {
	GetTaskChannelForConsume(taskType tasktype.TaskType) (chan taskentity.Task, error)
	GetTaskChannelForPublish(taskType tasktype.TaskType) (chan taskentity.Task, error)
	NewChannel(taskType tasktype.TaskType, bufferSize int)
}

type TaskManager struct {
	wg           *sync.WaitGroup
	done         <-chan bool
	adapter      Adapter
	eventManager eventmanager.EventManager
	taskService  taskservice.TaskService
}

func New(done <-chan bool, wg *sync.WaitGroup, adapter Adapter, taskService taskservice.TaskService,
	eventManager eventmanager.EventManager) (*TaskManager, error) {
	return &TaskManager{
		adapter:      adapter,
		wg:           wg,
		done:         done,
		taskService:  taskService,
		eventManager: eventManager,
	}, nil
}
func (tm *TaskManager) NewChannel(taskType tasktype.TaskType, bufferSize int) {
	tm.adapter.NewChannel(taskType, bufferSize)
}
func (tm *TaskManager) GetTaskChannelForConsume(taskType tasktype.TaskType) (<-chan taskentity.Task, error) {
	return tm.adapter.GetTaskChannelForConsume(taskType)
}
func (tm *TaskManager) GetTaskChannelForPublish(taskType tasktype.TaskType) (chan<- taskentity.Task, error) {
	return tm.adapter.GetTaskChannelForPublish(taskType)
}

func (tm *TaskManager) Start() {
	tm.wg.Add(1)
	go func() {
		defer tm.wg.Done()
		eventChannel := tm.eventManager.GetEventPublisherChannel()
		for {
			select {
			case <-tm.done:
				return
			case event := <-eventChannel:

				tm.wg.Add(1)
				go func(e event2.ProcessedEvent) {
					defer tm.wg.Done()
					taskType := tasktype.TaskType(e.DestinationType())
					targetChannel, err := tm.GetTaskChannelForPublish(taskType)
					if err != nil {
						slog.Error(err.Error())
						return
					}
					// TODO - We can check here if the event.ProcessedEvent.Integration is not present call manager to fill it
					taskID := e.ID()
					// Get task status using idempotency in the task service.
					taskStatus, err := tm.taskService.GetTaskStatusByID(context.Background(), taskID)
					if err != nil {
						slog.Error(err.Error())
						return
					}
					if !taskStatus.CanBeExecuted() {
						slog.Debug(fmt.Sprintf("Task [%s] has %s status and is not executable", taskID, taskStatus.String()))
						return
					}

					task := taskentity.MakeTaskUsingProcessedEvent(e)

					fmt.Println("Processed event received in task manager and task publish to TaskChannelForPublish")
					targetChannel <- task
				}(event)
			}
		}
	}()
}
