package rabbitmqeeventmanager

import (
	"github.com/ormushq/ormus/event"
	"sync"
)

type RabbitMQTaskManager struct {
	consumeChanel <-chan event.ProcessedEvent
	publishChanel chan<- event.ProcessedEvent
}

func New() (RabbitMQTaskManager, error) {
	return RabbitMQTaskManager{
		consumeChanel: make(<-chan event.ProcessedEvent),
		publishChanel: make(chan<- event.ProcessedEvent),
	}, nil
}

func (r RabbitMQTaskManager) GetEventChanelConsumer(done <-chan bool, wg *sync.WaitGroup) (*<-chan event.ProcessedEvent, error) {
	return &r.consumeChanel, nil
}

func (r RabbitMQTaskManager) GetEventChanelPublisher(done <-chan bool, wg *sync.WaitGroup) (*chan<- event.ProcessedEvent, error) {
	return &r.publishChanel, nil
}

func (r RabbitMQTaskManager) StartConsumer() {
	panic("not implemented")
}
func (r RabbitMQTaskManager) StartPublisher() {
	panic("not implemented")
}
