package main

import (
	"fmt"
	"github.com/ormushq/ormus/adapter/redis"
	"github.com/ormushq/ormus/destination/newimplementation/channel"
	rbbitmqadapter "github.com/ormushq/ormus/destination/newimplementation/channel/adapter/rabbitmq"
	tasktype "github.com/ormushq/ormus/destination/newimplementation/task"
	"github.com/ormushq/ormus/destination/newimplementation/taskmanager"
	"github.com/ormushq/ormus/destination/newimplementation/worker"
	"github.com/ormushq/ormus/destination/newimplementation/worker/handler/fakerworker"
	"github.com/ormushq/ormus/destination/newimplementation/workermanager"
	"github.com/ormushq/ormus/destination/taskservice"
	"github.com/ormushq/ormus/destination/taskservice/adapter/idempotency/redistaskidempotency"
	"github.com/ormushq/ormus/destination/taskservice/adapter/repository/inmemorytaskrepo"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/ormushq/ormus/config"
	"github.com/ormushq/ormus/logger"
)

/*
Flow of code
- Processed event publish from core service
- Processed event consume in event manager
- Event manager publish received event to local channel that task manager listens on it
- Task manager receives event from event manager
- Task manager convert event to task
- Task manager check task with task idempotency if it is not already handle continue
- Task manager publish event to provided publisherChannel from task manager adapter
- Worker listen on provided consumerChannel from task manager
- Worker receive task
- Worker check task with idempotency
- Worker handle task
- Worker store result in task repository
- Worker store task in task idempotency
- Worker publish deliver task response to provided channel from event manager

Overview of event flow
Event Manager => Task Manager => Worker
*/
func main() {
	done := make(chan bool)
	wg := sync.WaitGroup{}

	fileMaxSizeInMB := 10
	fileMaxAgeInDays := 30

	//------ Setup logger ------
	cfg := logger.Config{
		FilePath:         "./destination/logs.json",
		UseLocalTime:     false,
		FileMaxSizeInMB:  fileMaxSizeInMB,
		FileMaxAgeInDays: fileMaxAgeInDays,
	}

	logLevel := slog.LevelInfo
	if config.C().Destination.DebugMode {
		logLevel = slog.LevelDebug
	}

	opt := slog.HandlerOptions{
		// todo should level debug be read from config?
		Level: logLevel,
	}
	l := logger.New(cfg, &opt)
	slog.SetDefault(l)

	//Create new instant of redis addapter
	redisAdapter, err := redis.New(config.C().Redis)
	if err != nil {
		log.Panicf("error in new redis")
	}

	//Create task idempotency with redis adapter
	//This use for check if specific task already handled or not
	taskIdempotency := redistaskidempotency.New(redisAdapter, "tasks:", 30*24*time.Hour)
	//Creat task repo
	//This use for store result of task handle to DB
	taskRepo := inmemorytaskrepo.New()

	//Both task repo and task idempotency pass to task service and other services
	//use this for access to theme
	taskService := taskservice.New(taskIdempotency, taskRepo)

	// We define all worker here
	// for each task type we can register a worker instant
	workers := map[tasktype.TaskType]worker.Instant{
		tasktype.Fake:    fakerworker.New(done, &wg, taskService, 5),
		tasktype.Webhook: fakerworker.New(done, &wg, taskService, 5),
	}
	destinationConfig := config.C().Destination

	channelConverter := channel.NewConverter(done, &wg)
	//channelManager := channelmanager.New(done, &wg)
	channelAdapter := rbbitmqadapter.New(done, &wg, destinationConfig.RabbitmqConnection)
	//simpleChannelAdapter := simple.New(done, &wg)

	channelAdapter.NewChannel(
		destinationConfig.EventManager.RabbitMQEventManagerConnection.ProcessedEventChannelName,
		channel.OutputOnly,
		100, 5)

	channelAdapter.NewChannel(
		destinationConfig.EventManager.RabbitMQEventManagerConnection.DeliverTaskChannelName,
		channel.InputOnlyMode,
		100, 5)

	//channelManager.RegisterChannelAdapter("rabbitmq", rabbitmqChannelAdapter)

	// Event manager used for receive processed events from th core service
	// and pass them to task service
	// this adapter run multiple listeners on rabbitmq queue and it configurable
	// from RabbitMQEventManagerConnection
	//eventManager, eventManagerErr := eventmanager.New(done, &wg,
	//	destinationConfig.EventManager.RabbitMQEventManagerConnection,
	//	channelAdapter, 5)

	//if eventManagerErr != nil {
	//	log.Panicf("error in new event manager: %v", eventManagerErr)
	//}

	// Task manager adapter used for provide two channel
	// one for consume and another one for publish
	// The reason why it use two channels is that if we want to separate the workers
	// to another process and use a message broker in between theme we need to create
	// two channel one provide for publish task one provide for consume task
	// it flow like this
	//
	// task manager listen for events on event publisher channel
	// convert it to task
	// publish task to publish task channel
	// ------
	// worker process consume on consume task channel
	//
	// in one scenario both publisher channel and consumer is the same
	// in one scenario we can separate them two different channels
	// publisher channel receive task and pss it to broker
	// consumer process listen on rabbitmq and after receive task publish it on publish chanel
	//
	// This adapter init with 3 modes publisher, consumer, both
	// if we separate worker process we use consumer mode on worker process
	// and publisher mode on other processes
	// if we use only one binary we can use both mode
	//taskManagerAdapter, errCTM := rabbitmqtaskmanager.New(done, &wg,
	//	destinationConfig.TaskManager.RabbitMQConnection,
	//	taskmanager.BothMode, 5)

	// This adapter is simple and both consumer and publisher channels are same
	//taskManagerAdapter, errCTM := channeltaskmanager.New(done, &wg)

	//if errCTM != nil {
	//	log.Panicf("error in new taskManagerErr: %v", errCTM)
	//}

	// Task manager use one task adapter to handle tasks
	taskManager, taskManagerErr := taskmanager.New(done, &wg,
		taskmanager.TaskManagerParam{
			Config:                    destinationConfig.TaskManager,
			ChannelConverter:          channelConverter,
			ChannelAdapter:            channelAdapter,
			ChannelMode:               channel.BothMode,
			ProcessedEventChannelName: destinationConfig.EventManager.RabbitMQEventManagerConnection.ProcessedEventChannelName,
			TaskService:               taskService,
		},
	)
	if taskManagerErr != nil {
		log.Panicf("error in new taskManagerErr: %v", taskManagerErr)
	}

	// Worker manager managed the workers
	// it watches workers if they crash try to recreate them for specific times
	workerManager, workerManagerErr := workermanager.New(done, &wg, channelAdapter, channelConverter,
		destinationConfig.TaskManager.ChannelPrefix,
		destinationConfig.EventManager.RabbitMQEventManagerConnection.DeliverTaskChannelName)
	if workerManagerErr != nil {
		log.Panicf("error in new workerManager: %v", workerManagerErr)
	}

	// Here we register workers to the worker manager and create for them channels
	// every task types has it own channel
	for t, w := range workers {
		taskManager.NewChannel(t, 100, 5)
		registerWorkerErr := workerManager.RegisterWorker(t, w)
		if registerWorkerErr != nil {
			log.Panicf("error in new webhookWorkerErr: %v", registerWorkerErr)
		}
	}

	//We just start task manager and worker manager
	taskManager.Start()
	workerManager.Start()

	//----- Handling graceful shutdown  -----//

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit

	fmt.Println("Received interrupt signal, shutting down gracefully...")
	//done <- true

	close(done)

	// todo use config for waiting time after graceful shutdown
	time.Sleep(5 * time.Second)
	wg.Wait()
}
