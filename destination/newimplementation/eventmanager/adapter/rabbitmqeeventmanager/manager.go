package rabbitmqeeventmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ormushq/ormus/destination/dconfig"
	"github.com/ormushq/ormus/destination/entity/taskentity"
	"github.com/ormushq/ormus/destination/taskdelivery/param"
	"github.com/ormushq/ormus/event"
	"github.com/ormushq/ormus/pkg/errmsg"
	amqp "github.com/rabbitmq/amqp091-go"
	"log"
	"log/slog"
	"sync"
	"time"
)

type RabbitMQTaskManager struct {
	Id int
	// Consume processed events from rabbitmq and send them to eventPublisherChannel
	eventPublisherChannel chan event.ProcessedEvent
	// Get delivery task response from  deliveryTaskChannel and publish them to rabbitmq
	deliveryTaskChannel chan param.DeliveryTaskResponse
	rabbitmqConnection  *amqp.Connection
	wg                  *sync.WaitGroup
	done                <-chan bool
	config              dconfig.RabbitMQEventManagerConnection
}

func New(done <-chan bool, wg *sync.WaitGroup, config dconfig.RabbitMQEventManagerConnection) (RabbitMQTaskManager, error) {
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

	rabbitMQTaskManager := RabbitMQTaskManager{
		rabbitmqConnection:    conn,
		wg:                    wg,
		done:                  done,
		config:                config,
		eventPublisherChannel: make(chan event.ProcessedEvent, config.ProcessedEventChannelSize),
		deliveryTaskChannel:   make(chan param.DeliveryTaskResponse, config.DeliverEventChannelSize),
	}
	rabbitMQTaskManager.PrepareProcessedEventRabbitMq()
	rabbitMQTaskManager.PrepareDeliverEventRabbitMq()

	return rabbitMQTaskManager, nil
}

// GetEventPublisherChannel Return write only channel
func (r RabbitMQTaskManager) GetEventPublisherChannel() chan event.ProcessedEvent {
	return r.eventPublisherChannel
}

// GetDeliveryTaskChannel Return read only channel
func (r RabbitMQTaskManager) GetDeliveryTaskChannel() chan param.DeliveryTaskResponse {
	return r.deliveryTaskChannel
}

func (r RabbitMQTaskManager) PrepareProcessedEventRabbitMq() {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		ch, err := r.rabbitmqConnection.Channel()
		failOnError(err, errmsg.ErrToOpenChannel)
		defer func(ch *amqp.Channel) {
			err = ch.Close()
			failOnError(err, errmsg.ErrToCloseChannel)
		}(ch)

		err = ch.ExchangeDeclare(
			r.config.ProcessedEventTopic, // name
			"topic",                      // type
			true,                         // durable
			false,                        // auto-deleted
			false,                        // internal
			false,                        // no-wait
			nil,                          // arguments
		)
		failOnError(err, "Failed to declare an exchange")

		fromRabbitQueue, err := ch.QueueDeclare(
			r.config.ProcessedEventQueue, // name
			true,                         // durable
			false,                        // delete when unused
			true,                         // exclusive
			false,                        // no-wait
			nil,                          // arguments
		)
		failOnError(err, "Failed to declare a queue")

		err = ch.QueueBind(
			fromRabbitQueue.Name,         // queue name
			"",                           // routing key
			r.config.ProcessedEventTopic, // exchange
			false,
			nil)
		failOnError(err, "Failed to bind a queue")
		for i := 0; i < r.config.NumberInstants; i++ {
			r.wg.Add(1)
			go r.startConsumeProcessedEvent()
		}
	}()
}
func (r RabbitMQTaskManager) startConsumeProcessedEvent() {
	defer r.wg.Done()
	ch, err := r.rabbitmqConnection.Channel()
	failOnError(err, errmsg.ErrToOpenChannel)
	defer func(ch *amqp.Channel) {
		err = ch.Close()
		failOnError(err, errmsg.ErrToCloseChannel)
	}(ch)

	msgs, err := ch.Consume(
		r.config.ProcessedEventQueue, // queue
		"",                           // consumer
		false,                        // auto-ack
		false,                        // exclusive
		false,                        // no-local
		false,                        // no-wait
		nil,                          // arguments
	)
	failOnError(err, "failed to consume")

	for {
		select {
		case msg := <-msgs:
			if len(msg.Body) == 0 {
				continue
			}
			e, uErr := taskentity.UnmarshalBytesToProcessedEvent(msg.Body)
			if uErr != nil {
				slog.Error(fmt.Sprintf("Failed to convert bytes to processed events: %v", uErr))
				continue
			}

			fmt.Println("Processed event receive in event manager and publish to event publisher channel ")
			r.eventPublisherChannel <- e

			// Acknowledge the message
			err = msg.Ack(false)
			if err != nil {
				slog.Error(fmt.Sprintf("Failed to acknowledge message: %v", err))
			}
		case <-r.done:
			return
		}
	}

}

func (r RabbitMQTaskManager) PrepareDeliverEventRabbitMq() {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		ch, err := r.rabbitmqConnection.Channel()
		failOnError(err, errmsg.ErrToOpenChannel)
		defer func(ch *amqp.Channel) {
			err = ch.Close()
			failOnError(err, errmsg.ErrToCloseChannel)
		}(ch)

		err = ch.ExchangeDeclare(
			r.config.DeliverEventTopic, // name
			"topic",                    // type
			true,                       // durable
			false,                      // auto-deleted
			false,                      // internal
			false,                      // no-wait
			nil,                        // arguments
		)
		failOnError(err, "Failed to declare an exchange")

		toRabbitQueue, err := ch.QueueDeclare(
			r.config.DeliverEventQueue, // name
			false,                      // durable
			false,                      // delete when unused
			true,                       // exclusive
			false,                      // no-wait
			nil,                        // arguments
		)
		failOnError(err, "Failed to declare a queue")

		err = ch.QueueBind(
			toRabbitQueue.Name,         // queue name
			"",                         // routing key
			r.config.DeliverEventTopic, // exchange
			false,
			nil)
		failOnError(err, "Failed to bind a queue")

		for {
			select {
			case i := <-r.deliveryTaskChannel:

				jpe, err := json.Marshal(i)
				if err != nil {
					log.Panicf("Error: %e", err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), r.config.DeliverEventTimeoutInSeconds*time.Second)
				err = ch.PublishWithContext(ctx,
					r.config.DeliverEventTopic, // exchange
					"",                         // routing key
					false,                      // mandatory
					false,                      // immediate
					amqp.Publishing{
						ContentType: "text/plain",
						Body:        jpe,
					})
				failOnError(err, "Failed to publish a message")
				cancel()
			case <-r.done:

				return
			}
		}
	}()
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}
