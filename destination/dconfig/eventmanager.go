package dconfig

import "time"

type EventManager struct {
	RabbitMQEventManagerConnection RabbitMQEventManagerConnection `koanf:"rabbitmq_event_manager_connection"`
}

type RabbitMQEventManagerConnection struct {
	User                        string        `koanf:"user"`
	Password                    string        `koanf:"password"`
	Host                        string        `koanf:"host"`
	Port                        int           `koanf:"port"`
	Vhost                       string        `koanf:"vhost"`
	DeliverTaskChannelName      string        `koanf:"deliver_task_channel_name"`
	ProcessedEventChannelName   string        `koanf:"processed_event_channel_name"`
	DeliverTaskTopic            string        `koanf:"deliver_task_topic"`
	ProcessedEventTopic         string        `koanf:"processed_event_topic"`
	DeliverTaskTimeoutInSeconds time.Duration `koanf:"processed_event_timeout_in_seconds"`
	DeliverTaskQueue            string        `koanf:"deliver_task_queue"`
	ProcessedEventQueue         string        `koanf:"processed_event_queue"`
	DeliverTaskChannelSize      int           `koanf:"deliver_task_channel_size"`
	ProcessedEventChannelSize   int           `koanf:"processed_event_channel_size"`
	NumberInstants              int           `koanf:"number_instants"`
}
