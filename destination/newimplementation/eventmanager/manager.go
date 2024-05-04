package eventmanager

import (
	"encoding/json"
	"fmt"
	"github.com/ormushq/ormus/destination/dconfig"
	"github.com/ormushq/ormus/destination/entity/taskentity"
	"github.com/ormushq/ormus/destination/newimplementation/channel"
	"github.com/ormushq/ormus/destination/taskdelivery/param"
	"github.com/ormushq/ormus/event"
	"log"
	"log/slog"
	"sync"
)

type TaskManager struct {
	processedEventChannel chan event.ProcessedEvent
	deliveryTaskChannel   chan param.DeliveryTaskResponse
	config                dconfig.RabbitMQEventManagerConnection
	channelAdapter        channel.Adapter
	numberInstants        int
	wg                    *sync.WaitGroup
	done                  <-chan bool
}

func New(done <-chan bool, wg *sync.WaitGroup, config dconfig.RabbitMQEventManagerConnection,
	channelAdapter channel.Adapter, numberInstants int,
) (TaskManager, error) {
	taskManager := TaskManager{
		numberInstants:        numberInstants,
		wg:                    wg,
		done:                  done,
		config:                config,
		processedEventChannel: make(chan event.ProcessedEvent, 100),
		deliveryTaskChannel:   make(chan param.DeliveryTaskResponse, 100),
		channelAdapter:        channelAdapter,
	}
	taskManager.PrepareProcessedEvent()
	taskManager.PrepareDeliverEvent()

	return taskManager, nil
}

// GetEventPublisherChannel Return write only channel
func (t TaskManager) GetEventPublisherChannel() chan event.ProcessedEvent {
	return t.processedEventChannel
}

// GetDeliveryTaskChannel Return read only channel
func (t TaskManager) GetDeliveryTaskChannel() chan param.DeliveryTaskResponse {
	return t.deliveryTaskChannel
}

func (t TaskManager) PrepareProcessedEvent() {
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()

		for i := 0; i < t.numberInstants; i++ {
			t.wg.Add(1)
			go t.startConsumeProcessedEvent()
		}
	}()
}
func (t TaskManager) startConsumeProcessedEvent() {
	defer t.wg.Done()
	outputChannel, err := t.channelAdapter.GetOutputChannel(t.config.ProcessedEventChannelName)
	if err != nil {
		log.Panic(err)
		return
	}
	for {
		select {
		case msg := <-outputChannel:
			t.wg.Add(1)
			go func(msg []byte) {
				defer t.wg.Done()
				e, uErr := taskentity.UnmarshalBytesToProcessedEvent(msg)
				if uErr != nil {
					slog.Error(fmt.Sprintf("Failed to convert bytes to processed events: %v", uErr))
					return
				}

				slog.Debug(string(msg))
				t.processedEventChannel <- e
			}(msg)

		case <-t.done:
			return
		}
	}

}

func (t TaskManager) PrepareDeliverEvent() {
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		inputChannel, err := t.channelAdapter.GetInputChannel(t.config.ProcessedEventChannelName)
		if err != nil {
			log.Panic(err)
		}
		for {
			select {
			case i := <-t.deliveryTaskChannel:
				t.wg.Add(1)
				go func(deliverTask param.DeliveryTaskResponse) {
					defer t.wg.Done()
					jpe, err := json.Marshal(i)
					if err != nil {
						log.Panicf("Error: %e", err)
					}
					inputChannel <- jpe
				}(i)
			case <-t.done:

				return
			}
		}
	}()
}
