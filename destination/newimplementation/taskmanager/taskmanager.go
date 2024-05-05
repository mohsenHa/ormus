package taskmanager

import (
	"context"
	"fmt"
	"github.com/ormushq/ormus/destination/dconfig"
	"github.com/ormushq/ormus/destination/entity/taskentity"
	"github.com/ormushq/ormus/destination/newimplementation/channel"
	tasktype "github.com/ormushq/ormus/destination/newimplementation/task"
	"github.com/ormushq/ormus/destination/taskservice"
	event2 "github.com/ormushq/ormus/event"
	"log"
	"log/slog"
	"sync"
)

type Adapter interface {
	GetTaskChannelForConsume(taskType tasktype.TaskType) (chan taskentity.Task, error)
	GetTaskChannelForPublish(taskType tasktype.TaskType) (chan taskentity.Task, error)
	NewChannel(taskType tasktype.TaskType, bufferSize int)
}

type TaskManager struct {
	wg               *sync.WaitGroup
	done             <-chan bool
	config           dconfig.TaskManager
	channelMod       channel.Mode
	channelAdapter   channel.Adapter
	channelConverter *channel.Converter
	//eventManager      eventmanager.EventManager
	taskService               taskservice.TaskService
	processedEventChannelName string
	//taskOutputChannel         map[tasktype.TaskType]chan taskentity.Task
	//taskInputChannel          map[tasktype.TaskType]chan taskentity.Task
}
type TaskManagerParam struct {
	Config                    dconfig.TaskManager
	ChannelConverter          *channel.Converter
	ChannelAdapter            channel.Adapter
	ChannelMode               channel.Mode
	ProcessedEventChannelName string
	TaskService               taskservice.TaskService
}

func New(done <-chan bool, wg *sync.WaitGroup,
	param TaskManagerParam) (*TaskManager, error) {
	return &TaskManager{
		channelAdapter:            param.ChannelAdapter,
		channelConverter:          param.ChannelConverter,
		channelMod:                param.ChannelMode,
		config:                    param.Config,
		wg:                        wg,
		done:                      done,
		processedEventChannelName: param.ProcessedEventChannelName,
		taskService:               param.TaskService,
	}, nil
}

func (tm *TaskManager) GetTaskOutputChannel(taskType tasktype.TaskType) (<-chan taskentity.Task, error) {
	outputChannel, err := tm.channelAdapter.GetOutputChannel(tm.getTaskChannelName(taskType))
	if err != nil {
		return nil, fmt.Errorf("output channel not found: %v", taskType)
	}
	taskOutputChannel := tm.channelConverter.ConvertToOutputTaskChannel(outputChannel)
	return taskOutputChannel, nil
}
func (tm *TaskManager) GetTaskInputChannel(taskType tasktype.TaskType) (chan<- taskentity.Task, error) {
	inputChannel, err := tm.channelAdapter.GetInputChannel(tm.getTaskChannelName(taskType))
	if err != nil {
		return nil, fmt.Errorf("input channel not found: %v", taskType)
	}
	taskInputChannel := tm.channelConverter.ConvertToInputTaskChannel(inputChannel)
	return taskInputChannel, nil
}
func (tm *TaskManager) getTaskChannelName(taskType tasktype.TaskType) string {
	return GetTaskChannelName(tm.config.ChannelPrefix, taskType)
}

func (tm *TaskManager) Start() {
	tm.wg.Add(1)
	go func() {
		defer tm.wg.Done()
		c, err := tm.channelAdapter.GetOutputChannel(tm.processedEventChannelName)
		eventChannel := tm.channelConverter.ConvertToOutputProcessedEventChannel(c)
		if err != nil {
			log.Panic(err)
		}
		for {
			select {
			case <-tm.done:
				return
			case event := <-eventChannel:

				tm.wg.Add(1)
				go tm.handleEvent(event)
			}
		}
	}()
}

func (tm *TaskManager) handleEvent(e event2.ProcessedEvent) {
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
		fmt.Println("destination/newimplementation/taskmanager/taskmanager.go:118",
			fmt.Sprintf("Task [%s] has %s status and is not executable", taskID, taskStatus.String()))
		return
	}

	task := taskentity.MakeTaskUsingProcessedEvent(e)
	fmt.Println("destination/newimplementation/taskmanager/taskmanager.go:124", fmt.Sprintf("%+v", task))
	targetChannel <- task
}

func GetTaskChannelName(ChannelPrefix string, taskType tasktype.TaskType) string {
	return ChannelPrefix + string(taskType)
}
