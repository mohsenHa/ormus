package dconfig

import "time"

type EventManager struct {
	RabbitMQEventManagerConnection RabbitMQEventManagerConnection `koanf:"rabbitmq_event_manager_connection"`
}

type RabbitMQEventManagerConnection struct {
	User                         string        `koanf:"user"`
	Password                     string        `koanf:"password"`
	Host                         string        `koanf:"host"`
	Port                         int           `koanf:"port"`
	Vhost                        string        `koanf:"vhost"`
	DeliverEventTopic            string        `koanf:"deliver_event_topic"`
	ProcessedEventTopic          string        `koanf:"processed_event_topic"`
	DeliverEventTimeoutInSeconds time.Duration `koanf:"processed_event_timeout_in_seconds"`
	DeliverEventQueue            string        `koanf:"deliver_event_queue"`
	ProcessedEventQueue          string        `koanf:"processed_event_queue"`
	DeliverEventChannelSize      int           `koanf:"deliver_event_channel_size"`
	ProcessedEventChannelSize    int           `koanf:"processed_event_channel_size"`
	NumberInstants               int           `koanf:"number_instants"`
}
