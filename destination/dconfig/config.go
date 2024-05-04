package dconfig

type Config struct {
	DebugMode                  bool                       `koanf:"debug_mode"`
	RabbitMQConsumerConnection RabbitMQConsumerConnection `koanf:"rabbitmq_consumer_connection"`
	ConsumerTopic              ConsumerTopic              `koanf:"consumer_topic"`
	RedisTaskIdempotency       RedisTaskIdempotency       `koanf:"redis_idempotency"`
	EventManager               EventManager               `koanf:"event_manager"`
	TaskManager                TaskManager                `koanf:"task_manager"`
	RabbitmqConnection         RabbitmqConnection         `koanf:"rabbitmq_connection"`
}

type RabbitmqConnection struct {
	User     string `koanf:"user"`
	Password string `koanf:"password"`
	Host     string `koanf:"host"`
	Port     int    `koanf:"port"`
	Vhost    string `koanf:"vhost"`
}
