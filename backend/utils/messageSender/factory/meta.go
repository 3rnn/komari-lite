package factory

import "github.com/komari-monitor/komari/database/models"

type IMessageSender interface {
	GetName() string
	// Be sure to return a pointer to &Configuration{}
	GetConfiguration() Configuration
	SendTextMessage(message, title string) error
	Init() error
	Destroy() error
}

// IEventMessageSender is an optional interface that can receive structured event messages if implemented.
type IEventMessageSender interface {
	SendEvent(event models.EventMessage) error
}

type Configuration interface{}

type MessageSenderConstructor func() IMessageSender
