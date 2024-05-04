package rbbitmqadapter

import (
	"context"
	"fmt"
	"github.com/ormushq/ormus/destination/dconfig"
	"github.com/ormushq/ormus/destination/newimplementation/channel"
	amqp "github.com/rabbitmq/amqp091-go"
	"log/slog"
	"sync"
)

type rabbitmqChannel struct {
	wg               *sync.WaitGroup
	done             <-chan bool
	mode             channel.Mode
	config           dconfig.RabbitmqConnection
	rabbitConnection *amqp.Connection
	inputChannel     chan []byte
	outputChannel    chan []byte
	exchange         string
	queue            string
	numberInstants   int
}
type rabbitmqChannelParams struct {
	mode           channel.Mode
	config         dconfig.RabbitmqConnection
	exchange       string
	queue          string
	bufferSize     int
	numberInstants int
}

func newChannel(done <-chan bool, wg *sync.WaitGroup, rabbitmqChannelParams rabbitmqChannelParams) *rabbitmqChannel {
	conn, err := amqp.Dial(fmt.Sprintf("amqp://%s:%s@%s:%d/%s",
		rabbitmqChannelParams.config.User, rabbitmqChannelParams.config.Password, rabbitmqChannelParams.config.Host,
		rabbitmqChannelParams.config.Port, rabbitmqChannelParams.config.Vhost))
	failOnError(err, "Failed to connect to rabbitmq server")

	ch, err := conn.Channel()
	failOnError(err, "failed to open a channel")
	err = ch.ExchangeDeclare(
		rabbitmqChannelParams.exchange, // name
		"topic",                        // type
		true,                           // durable
		false,                          // auto-deleted
		false,                          // internal
		false,                          // no-wait
		nil,                            // arguments
	)
	failOnError(err, "Failed to declare an exchange")

	_, errQueueDeclare := ch.QueueDeclare(
		rabbitmqChannelParams.queue, // name
		true,                        // durable
		false,                       // delete when unused
		true,                        // exclusive
		false,                       // no-wait
		nil,                         // arguments
	)
	failOnError(errQueueDeclare, "Failed to declare a queue")
	errQueueBind := ch.QueueBind(
		rabbitmqChannelParams.queue,    // queue name
		"",                             // routing key
		rabbitmqChannelParams.exchange, // exchange
		false,
		nil)
	failOnError(errQueueBind, "Failed to bind a queue")

	rc := &rabbitmqChannel{
		done:             done,
		wg:               wg,
		mode:             rabbitmqChannelParams.mode,
		config:           rabbitmqChannelParams.config,
		exchange:         rabbitmqChannelParams.exchange,
		queue:            rabbitmqChannelParams.queue,
		rabbitConnection: conn,
		numberInstants:   rabbitmqChannelParams.numberInstants,
		inputChannel:     make(chan []byte, rabbitmqChannelParams.bufferSize),
		outputChannel:    make(chan []byte, rabbitmqChannelParams.bufferSize),
	}
	rc.startInput()
	rc.startOutput()
	return rc
}
func (rc rabbitmqChannel) GetMode() channel.Mode {
	return rc.mode
}
func (rc rabbitmqChannel) GetInputChannel() chan<- []byte {
	return rc.inputChannel
}
func (rc rabbitmqChannel) GetOutputChannel() <-chan []byte {
	return rc.outputChannel
}
func (rc rabbitmqChannel) startInput() {
	if !rc.mode.IsInputMode() {
		return
	}

	for i := 0; i < rc.numberInstants; i++ {
		rc.wg.Add(1)
		go func() {
			defer rc.wg.Done()
			ch, err := rc.rabbitConnection.Channel()
			failOnError(err, "Failed to open a channel")
			defer func(ch *amqp.Channel) {
				err = ch.Close()
				failOnError(err, "Failed to close channel")
			}(ch)

			for {
				select {
				case <-rc.done:
					return
				case msg := <-rc.inputChannel:
					slog.Debug(string(msg))
					go func(msg []byte) {
						defer rc.wg.Done()

						errPWC := ch.PublishWithContext(context.Background(),
							rc.exchange, // exchange
							"",          // routing key
							false,       // mandatory
							false,       // immediate
							amqp.Publishing{
								ContentType: "text/plain",
								Body:        msg,
							})
						if errPWC != nil {
							slog.Error(errPWC.Error())
							return
						}
					}(msg)
				}
			}
		}()
	}
}
func (rc rabbitmqChannel) startOutput() {
	if !rc.mode.IsOutputMode() {
		return
	}
	for i := 0; i < rc.numberInstants; i++ {
		rc.wg.Add(1)
		go func() {
			defer rc.wg.Done()
			ch, err := rc.rabbitConnection.Channel()
			failOnError(err, "Failed to open a channel")
			defer func(ch *amqp.Channel) {
				err = ch.Close()
				failOnError(err, "Failed to close channel")
			}(ch)
			msgs, errConsume := ch.Consume(
				rc.queue, // queue
				"",       // consumer
				false,    // auto-ack
				false,    // exclusive
				false,    // no-local
				false,    // no-wait
				nil,      // arguments
			)
			failOnError(errConsume, "failed to consume")
			for {
				select {
				case <-rc.done:
					return
				case msg := <-msgs:
					slog.Debug(string(msg.Body))
					rc.outputChannel <- msg.Body
				}
			}
		}()
	}
}

func failOnError(err error, msg string) {
	if err != nil {
		fmt.Println(msg)
		panic(err)
	}
}