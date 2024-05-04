package fakerworker

import (
	"context"
	"fmt"
	"github.com/ormushq/ormus/destination/entity/taskentity"
	"github.com/ormushq/ormus/destination/newimplementation/worker"
	"github.com/ormushq/ormus/destination/taskdelivery/param"
	"github.com/ormushq/ormus/destination/taskservice"
	"log/slog"
	"sync"
	"time"
)

type Worker struct {
	taskService taskservice.TaskService
	wg          *sync.WaitGroup
	done        <-chan bool
	crashCount  uint
}

func New(done <-chan bool, wg *sync.WaitGroup, taskService taskservice.TaskService,
	numberOfInstants int) worker.Instant {
	return worker.Instant{
		Worker: &Worker{
			taskService: taskService,
			wg:          wg,
			done:        done,
			crashCount:  0,
		},
		NumberOfInstants: numberOfInstants,
	}
}

func (w *Worker) Work(channel <-chan taskentity.Task, deliverChannel chan<- param.DeliveryTaskResponse, crashChan chan<- uint) {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()

		// If worker process is crashed then recreate worker
		defer func() {
			if err := recover(); err != nil {
				slog.Error(
					fmt.Sprintf("worker is crashed try to restart worker retry count%d. err:%v", w.crashCount, err))
				crashChan <- w.crashCount
			}
		}()

		for {
			select {
			case task := <-channel:
				w.wg.Add(1)
				fmt.Println(task)
				go w.taskReceive(task, deliverChannel)
			case <-w.done:
				return

			}
		}
	}()
}
func (w *Worker) taskReceive(task taskentity.Task, deliverChannel chan<- param.DeliveryTaskResponse) {
	defer w.wg.Done()
	taskStatus, err := w.taskService.GetTaskStatusByID(context.Background(), task.ID)
	if err != nil {
		slog.Error(err.Error())
		return
	}

	if !taskStatus.CanBeExecuted() {
		fmt.Println("destination/newimplementation/worker/handler/fakerworker/worker.go:71",
			fmt.Sprintf("Task [%s] has %s status and is not executable", task.ID,
				taskStatus.String()))
		return
	}

	deliveryResponse, err := w.handle(task)
	if err != nil {
		slog.Error(fmt.Sprintf("Task [%s] not handled error: %s!", task.ID, err))
		return
	}
	// DeliveryStatus is set in delivery handler in case of success, retriable failed, unretriable failed.
	task.IntegrationDeliveryStatus = deliveryResponse.DeliveryStatus
	// Attempts is incremented in delivery handler in case of success.
	task.Attempts = deliveryResponse.Attempts
	// FailedReason describes what caused the failure.
	task.FailedReason = deliveryResponse.FailedReason

	err = w.taskService.UpsertTaskAndSaveIdempotency(context.Background(), task)
	if err != nil {
		slog.Error(err.Error())
		return
	}
	deliverChannel <- deliveryResponse
}
func (w *Worker) UpdateCrashCount(lastCrashCount uint) {
	w.crashCount = lastCrashCount + 1
}
func (w *Worker) handle(t taskentity.Task) (param.DeliveryTaskResponse, error) {
	time.Sleep(10 * time.Second)

	slog.Info(fmt.Sprintf("Task [%s] handled successfully!", t.ID))

	res := param.DeliveryTaskResponse{
		Attempts:       1,
		FailedReason:   nil,
		DeliveryStatus: taskentity.SuccessTaskStatus,
	}
	return res, nil

}
