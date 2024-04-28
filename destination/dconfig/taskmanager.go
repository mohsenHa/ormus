package dconfig

type TaskManager struct {
	RabbitMQConnection RabbitMQTaskManagerConnection `koanf:"rabbitmq_task_manager_connection"`
}
type RabbitMQTaskManagerConnection struct {
	User           string `koanf:"user"`
	Password       string `koanf:"password"`
	Host           string `koanf:"host"`
	Port           int    `koanf:"port"`
	Vhost          string `koanf:"vhost"`
	QueuePrefix    string `koanf:"queue_prefix"`
	ExchangePrefix string `koanf:"exchange_prefix"`
}
