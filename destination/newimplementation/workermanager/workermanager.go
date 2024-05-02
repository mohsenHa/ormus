package workermanager

import (
	"fmt"
	"github.com/ormushq/ormus/destination/newimplementation/eventmanager"
	tasktype "github.com/ormushq/ormus/destination/newimplementation/task"
	"github.com/ormushq/ormus/destination/newimplementation/taskmanager"
	"github.com/ormushq/ormus/destination/newimplementation/worker"
	"log/slog"
	"sync"
)

type WorkerManager struct {
	workers      map[tasktype.TaskType]worker.Instant
	taskManager  *taskmanager.TaskManager
	eventManager eventmanager.EventManager
	wg           *sync.WaitGroup
	done         <-chan bool
}

func New(done <-chan bool, wg *sync.WaitGroup,
	eventManager eventmanager.EventManager, taskManager *taskmanager.TaskManager) (*WorkerManager, error) {
	return &WorkerManager{
		workers:      make(map[tasktype.TaskType]worker.Instant),
		taskManager:  taskManager,
		eventManager: eventManager,
		done:         done,
		wg:           wg,
	}, nil
}
func (wm *WorkerManager) RegisterWorker(taskType tasktype.TaskType, worker worker.Instant) error {
	wm.workers[taskType] = worker
	return nil
}

func (wm *WorkerManager) Start() {
	for t, w := range wm.workers {
		wm.startWorkers(w, t)
	}
}

func (wm *WorkerManager) startWorkers(workerInstant worker.Instant, taskType tasktype.TaskType) {
	channel, err := wm.taskManager.GetTaskChannelForConsume(taskType)
	if err != nil {
		slog.Error(fmt.Sprintf("For task type %v channel not found", taskType))
	}

	for i := 0; i < workerInstant.NumberOfInstants; i++ {
		wm.wg.Add(1)
		go func(cw worker.Worker, ct tasktype.TaskType) {
			defer wm.wg.Done()

			crashChan := make(chan uint)
			cw.Work(channel, wm.eventManager.GetDeliveryTaskChannel(), crashChan)
			for {
				select {
				case crashCount := <-crashChan:
					if crashCount > 5 {
						slog.Error(fmt.Sprintf("Worker for type %s is crashed mor than 5 time", ct))
					}
					cw.UpdateCrashCount(crashCount + 1)
					cw.Work(channel, wm.eventManager.GetDeliveryTaskChannel(), crashChan)
				case <-wm.done:
					return
				}
			}
		}(workerInstant.Worker, taskType)
	}

}
