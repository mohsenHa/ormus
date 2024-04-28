package tasktype

type TaskType string

const (
	Fake    TaskType = "fake"
	Webhook TaskType = "webhook"
)
