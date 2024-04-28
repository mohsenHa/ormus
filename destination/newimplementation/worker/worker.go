package worker

import (
	"github.com/ormushq/ormus/destination/entity/taskentity"
	"github.com/ormushq/ormus/destination/taskdelivery/param"
)

type Worker interface {
	Work(channel <-chan taskentity.Task, deliverChannel chan<- param.DeliveryTaskResponse, crashChannel chan<- uint)
	UpdateCrashCount(lastCrashCount uint)
}

type Instant struct {
	Worker           Worker
	NumberOfInstants int
}
