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

	channelAdapter := rbbitmqadapter.New(done, &wg, destinationConfig.RabbitmqConnection)

	channelAdapter.NewChannel(
		destinationConfig.EventManager.RabbitMQEventManagerConnection.DeliverTaskChannelName,
		channel.InputOnlyMode,
		100, 5)

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
		channelAdapter.NewChannel(taskmanager.GetTaskChannelName(destinationConfig.TaskManager.ChannelPrefix,
			t), channel.OutputOnly, 100, 5)
		registerWorkerErr := workerManager.RegisterWorker(t, w)
		if registerWorkerErr != nil {
			log.Panicf("error in new webhookWorkerErr: %v", registerWorkerErr)
		}
	}

	//We just start worker manager
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
