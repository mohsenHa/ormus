package taskmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ormushq/ormus/destination/dconfig"
	"github.com/ormushq/ormus/destination/entity/taskentity"
	"github.com/ormushq/ormus/destination/newimplementation/channel"
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
	wg                *sync.WaitGroup
	done              <-chan bool
	config            dconfig.TaskManager
	channelMod        channel.Mode
	channelAdapter    channel.Adapter
	eventManager      eventmanager.EventManager
	taskService       taskservice.TaskService
	taskOutputChannel map[tasktype.TaskType]chan taskentity.Task
	taskInputChannel  map[tasktype.TaskType]chan taskentity.Task
}

func New(done <-chan bool, wg *sync.WaitGroup, config dconfig.TaskManager, channelAdapter channel.Adapter, channelMode channel.Mode,
	taskService taskservice.TaskService, eventManager eventmanager.EventManager) (*TaskManager, error) {
	return &TaskManager{
		channelAdapter:    channelAdapter,
		channelMod:        channelMode,
		config:            config,
		wg:                wg,
		done:              done,
		taskService:       taskService,
		eventManager:      eventManager,
		taskOutputChannel: make(map[tasktype.TaskType]chan taskentity.Task),
		taskInputChannel:  make(map[tasktype.TaskType]chan taskentity.Task),
	}, nil
}
func (tm *TaskManager) NewChannel(taskType tasktype.TaskType, bufferSize int, numberInstants int) {
	tm.channelAdapter.NewChannel(tm.getTaskChannelName(taskType), tm.channelMod, bufferSize, numberInstants)
	tm.taskOutputChannel[taskType] = make(chan taskentity.Task, bufferSize)
	tm.taskInputChannel[taskType] = make(chan taskentity.Task, bufferSize)
}

func (tm *TaskManager) prepareOutputChannel(taskType tasktype.TaskType) {
	outputChannel, _ := tm.channelAdapter.GetOutputChannel(tm.getTaskChannelName(taskType))
	tm.wg.Add(1)
	go func() {
		defer tm.wg.Done()
		for {
			select {
			case <-tm.done:
				return
			case msq := <-outputChannel:
				tm.wg.Add(1)
				go func(msg []byte) {
					e, uErr := taskentity.UnmarshalBytesToTask(msg)
					if uErr != nil {
						slog.Error(fmt.Sprintf("Failed to convert bytes to processed events: %v", uErr))
						return
					}
					slog.Debug(string(msg))
					tm.taskOutputChannel[taskType] <- e
				}(msq)

			}
		}
	}()
}
func (tm *TaskManager) prepareInputChannel(taskType tasktype.TaskType) {
	inputChannel, _ := tm.channelAdapter.GetInputChannel(tm.getTaskChannelName(taskType))
	tm.wg.Add(1)
	go func() {
		defer tm.wg.Done()
		for {
			select {
			case <-tm.done:
				return
			case task := <-tm.taskInputChannel[taskType]:
				tm.wg.Add(1)
				go func(task taskentity.Task) {
					jpe, errM := json.Marshal(task)
					if errM != nil {
						slog.Error("Error: %e", errM)
						return
					}
					slog.Debug(string(jpe))
					inputChannel <- jpe
				}(task)

			}
		}
	}()
}

func (tm *TaskManager) GetTaskOutputChannel(taskType tasktype.TaskType) (<-chan taskentity.Task, error) {
	if c, ok := tm.taskOutputChannel[taskType]; ok {
		return c, nil
	}
	return nil, fmt.Errorf("output channel not found for task type %v", taskType)
}
func (tm *TaskManager) GetTaskInputChannel(taskType tasktype.TaskType) (chan<- taskentity.Task, error) {
	if c, ok := tm.taskInputChannel[taskType]; ok {
		return c, nil
	}
	return nil, fmt.Errorf("output channel not found for task type %v", taskType)
}
func (tm *TaskManager) getTaskChannelName(taskType tasktype.TaskType) string {
	return tm.config.ChannelPrefix + string(taskType)
}

func (tm *TaskManager) Start() {
	for taskType := range tm.taskInputChannel {
		go tm.prepareInputChannel(taskType)
		go tm.prepareOutputChannel(taskType)
	}
	//
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
					targetChannel, err := tm.GetTaskInputChannel(taskType)
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
					slog.Debug(fmt.Sprintf("%+v", task))
					targetChannel <- task
				}(event)
			}
		}
	}()
}