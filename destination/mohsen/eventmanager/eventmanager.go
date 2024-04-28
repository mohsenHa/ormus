package eventmanager

import (
	"github.com/ormushq/ormus/event"
	"sync"
)

type EventManager interface {
	Start()
	GetEventChanelConsumer(done <-chan bool, wg *sync.WaitGroup) (*<-chan event.ProcessedEvent, error)
	GetEventChanelPublisher(done <-chan bool, wg *sync.WaitGroup) (*chan<- event.ProcessedEvent, error)
}
