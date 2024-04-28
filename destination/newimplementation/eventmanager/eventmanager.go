package eventmanager

import (
	"github.com/ormushq/ormus/destination/taskdelivery/param"
	"github.com/ormushq/ormus/event"
)

type EventManager interface {
	GetDeliveryTaskChannel() chan param.DeliveryTaskResponse
	GetEventPublisherChannel() chan event.ProcessedEvent
}
