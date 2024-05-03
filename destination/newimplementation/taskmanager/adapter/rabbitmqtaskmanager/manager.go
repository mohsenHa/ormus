package rabbitmqtaskmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ormushq/ormus/destination/dconfig"
	"github.com/ormushq/ormus/destination/entity/taskentity"
	tasktype "github.com/ormushq/ormus/destination/newimplementation/task"
	"github.com/ormushq/ormus/pkg/errmsg"
	amqp "github.com/rabbitmq/amqp091-go"
	"log"
	"log/slog"
	"sync"
)

type Mode string

const (
	ConsumerMod   Mode = "consumer"
	PublisherMode Mode = "publisher"
	BothMode      Mode = "both"
)

type Manager struct {
	rabbitConnection       *amqp.Connection
	taskChannelsForConsume map[tasktype.TaskType]chan taskentity.Task
	taskChannelsForPublish map[tasktype.TaskType]chan taskentity.Task
	numberInstants         int
	mode                   Mode
	config                 dconfig.RabbitMQTaskManagerConnection
	wg                     *sync.WaitGroup
	done                   <-chan bool
}

func New(done <-chan bool, wg *sync.WaitGroup,
	config dconfig.RabbitMQTaskManagerConnection, mode Mode, numberInstants int) (*Manager, error) {
	conn, err := amqp.Dial(fmt.Sprintf("amqp://%s:%s@%s:%d/%s",
		config.User, config.Password, config.Host, config.Port, config.Vhost))
	failOnError(err, "Failed to connect to RabbitMQ")

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				err = conn.Close()
				failOnError(err, "Failed to close RabbitMQ connection")
				return
			}
		}
	}()

	return &Manager{
		rabbitConnection:       conn,
		taskChannelsForPublish: make(map[tasktype.TaskType]chan taskentity.Task),
		taskChannelsForConsume: make(map[tasktype.TaskType]chan taskentity.Task),
		numberInstants:         numberInstants,
		mode:                   mode,
		wg:                     wg,
		done:                   done,
		config:                 config,
	}, nil
}

func (m *Manager) NewChannel(taskType tasktype.TaskType, bufferSize int) {
	if m.isPublisherMode() {
		m.taskChannelsForPublish[taskType] = make(chan taskentity.Task, bufferSize)
		m.configurePublisherChannel(taskType)
	}
	if m.isConsumerMode() {
		m.taskChannelsForConsume[taskType] = make(chan taskentity.Task, bufferSize)
		m.configureConsumerChannel(taskType)
	}
}

func (m *Manager) configureConsumerChannel(taskType tasktype.TaskType) {
	ch, err := m.rabbitConnection.Channel()
	failOnError(err, errmsg.ErrToOpenChannel)
	defer func(ch *amqp.Channel) {
		err = ch.Close()
		failOnError(err, errmsg.ErrToCloseChannel)
	}(ch)
	err = ch.ExchangeDeclare(
		m.config.ExchangePrefix+string(taskType), // name
		"topic", // type
		true,    // durable
		false,   // auto-deleted
		false,   // internal
		false,   // no-wait
		nil,     // arguments
	)
	failOnError(err, "Failed to declare an exchange")

	fromRabbitQueue, err := ch.QueueDeclare(
		m.config.QueuePrefix+string(taskType), // name
		true,                                  // durable
		false,                                 // delete when unused
		true,                                  // exclusive
		false,                                 // no-wait
		nil,                                   // arguments
	)
	failOnError(err, "Failed to declare a queue")
	err = ch.QueueBind(
		fromRabbitQueue.Name, // queue name
		"",                   // routing key
		m.config.ExchangePrefix+string(taskType), // exchange
		false,
		nil)
	failOnError(err, "Failed to bind a queue")
	for i := 0; i < m.numberInstants; i++ {
		m.wg.Add(1)
		go m.startConsumerChannel(taskType)
	}
}
func (m *Manager) startConsumerChannel(taskType tasktype.TaskType) {
	defer m.wg.Done()
	ch, err := m.rabbitConnection.Channel()
	failOnError(err, errmsg.ErrToOpenChannel)
	defer func(ch *amqp.Channel) {
		err = ch.Close()
		failOnError(err, errmsg.ErrToCloseChannel)
	}(ch)
	taskChannel := m.taskChannelsForConsume[taskType]
	msgs, err := ch.Consume(
		m.config.QueuePrefix+string(taskType), // queue
		"",                                    // consumer
		false,                                 // auto-ack
		false,                                 // exclusive
		false,                                 // no-local
		false,                                 // no-wait
		nil,                                   // arguments
	)
	failOnError(err, "failed to consume")

	for {
		select {
		case msg := <-msgs:
			m.wg.Add(1)
			go func(msg amqp.Delivery) {
				defer m.wg.Done()

				if len(msg.Body) == 0 {
					return
				}
				e, uErr := taskentity.UnmarshalBytesToTask(msg.Body)
				if uErr != nil {
					slog.Error(fmt.Sprintf("Failed to convert bytes to processed events: %v", uErr))
					return
				}
				fmt.Println("Task receive in task manager consumer and publish to taskChannelsForConsume")

				taskChannel <- e

				// Acknowledge the message
				err = msg.Ack(false)
				if err != nil {
					slog.Error(fmt.Sprintf("Failed to acknowledge message: %v", err))
				}
			}(msg)
		case <-m.done:

			return
		}
	}
}

func (m *Manager) configurePublisherChannel(taskType tasktype.TaskType) {
	ch, err := m.rabbitConnection.Channel()
	failOnError(err, "Failed to open a channel")
	defer func(ch *amqp.Channel) {
		err = ch.Close()
		failOnError(err, "Failed to close channel")
	}(ch)
	err = ch.ExchangeDeclare(
		m.config.ExchangePrefix+string(taskType), // name
		"topic", // type
		true,    // durable
		false,   // auto-deleted
		false,   // internal
		false,   // no-wait
		nil,     // arguments
	)
	failOnError(err, "Failed to declare an exchange")

	for i := 0; i < m.numberInstants; i++ {
		m.wg.Add(1)
		go m.startPublisherChannel(taskType)
	}
}
func (m *Manager) startPublisherChannel(taskType tasktype.TaskType) {
	defer m.wg.Done()
	ch, err := m.rabbitConnection.Channel()
	failOnError(err, "Failed to open a channel")
	defer func(ch *amqp.Channel) {
		err = ch.Close()
		failOnError(err, "Failed to close channel")
	}(ch)
	channel, err := m.GetTaskChannelForPublish(taskType)
	failOnError(err, "Failed to get publish channel for task type")
	for {
		select {
		case task := <-channel:
			m.wg.Add(1)
			go func(task taskentity.Task) {
				defer m.wg.Done()
				jpe, errM := json.Marshal(task)
				if errM != nil {
					slog.Error("Error: %e", err)
					return
				}
				fmt.Println("Task receive in task manager publisher and publish to TaskChannelForPublish")

				errPWC := ch.PublishWithContext(context.Background(),
					m.config.ExchangePrefix+string(taskType), // exchange
					"",    // routing key
					false, // mandatory
					false, // immediate
					amqp.Publishing{
						ContentType: "text/plain",
						Body:        jpe,
					})
				if errPWC != nil {
					slog.Error(err.Error())
					return
				}
			}(task)
		case <-m.done:
			return
		}
	}
}

func (m *Manager) GetTaskChannelForConsume(taskType tasktype.TaskType) (chan taskentity.Task, error) {
	channel, ok := m.taskChannelsForConsume[taskType]
	if !ok {
		return nil, fmt.Errorf("task channel not found: %v", taskType)
	}
	return channel, nil
}

func (m *Manager) GetTaskChannelForPublish(taskType tasktype.TaskType) (chan taskentity.Task, error) {
	channel, ok := m.taskChannelsForPublish[taskType]
	if !ok {
		return nil, fmt.Errorf("task channel not found: %v", taskType)
	}
	return channel, nil
}

func (m *Manager) isPublisherMode() bool {
	return m.mode == PublisherMode || m.mode == BothMode
}
func (m *Manager) isConsumerMode() bool {
	return m.mode == ConsumerMod || m.mode == BothMode
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}
