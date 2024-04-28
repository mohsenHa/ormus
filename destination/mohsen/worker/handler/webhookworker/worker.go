package webhookworker

type WebhookWorker struct{}

func New() (*WebhookWorker, error) {
	return &WebhookWorker{}, nil
}

func (w *WebhookWorker) GetWorkerInstance() bool {
	return true
}
