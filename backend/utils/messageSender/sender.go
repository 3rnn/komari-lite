package messageSender

import (
	"encoding/json"
	"fmt"
	logger "github.com/komari-monitor/komari/utils/log"
	"strings"
	"sync"
	"time"

	"github.com/komari-monitor/komari/database"
	"github.com/komari-monitor/komari/database/auditlog"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/config"
	"github.com/komari-monitor/komari/utils/messageSender/factory"
)

var (
	currentProvider factory.IMessageSender
	mu              = sync.Mutex{}
	once            = sync.Once{}
)

func CurrentProvider() factory.IMessageSender {
	mu.Lock()
	defer mu.Unlock()
	return currentProvider
}

// Shutdown destroys the current message sending provider and releases the resources it holds. Called by the shutdown process.
func Shutdown() error {
	mu.Lock()
	defer mu.Unlock()
	if currentProvider == nil {
		return nil
	}
	err := currentProvider.Destroy()
	currentProvider = nil
	return err
}

func Initialize() {
	go func() {
		once.Do(func() {
			all := factory.GetAllMessageSenders()
			for _, provider := range all {
				if _, err := database.GetMessageSenderConfigByName(provider.GetName()); err == nil {
					continue
				}
				// If there is no configuration for this provider in the database, the default configuration is saved
				config := provider.GetConfiguration()
				configBytes, err := json.Marshal(config)
				if err != nil {
					logger.Errorf("message-sender", "Failed to marshal config for provider %s: %v", provider.GetName(), err)
					return
				}
				if err := database.SaveMessageSenderConfig(&models.MessageSenderProvider{
					Name:     provider.GetName(),
					Addition: string(configBytes),
				}); err != nil {
					logger.Errorf("message-sender", "Failed to save default config for provider %s: %v", provider.GetName(), err)
					return
				}
			}
		})
	}()
	NotificationMethod, _ := config.GetAs[string](config.NotificationMethodKey, "none")

	if NotificationMethod == "" || NotificationMethod == "none" {
		LoadProvider("empty", "{}")
		return
	}

	// Try loading configuration from database
	senderConfig, err := database.GetMessageSenderConfigByName(NotificationMethod)
	if err != nil {
		// If no configuration is found, use empty provider
		LoadProvider("empty", "{}")
		return
	}
	// The streamlined build may have removed the provider; fallback to empty when loading fails to avoid leaving the notification channel in an uninitialized state.
	if err := LoadProvider(NotificationMethod, senderConfig.Addition); err != nil {
		logger.Errorf("message-sender", "Failed to load provider %s: %v", NotificationMethod, err)
		LoadProvider("empty", "{}")
	}
}

func SendTextMessage(message string, title string) error {
	if CurrentProvider() == nil {
		return fmt.Errorf("message sender provider is not initialized")
	}
	var err error
	NotificationEnabled, err := config.GetAs[bool](config.NotificationEnabledKey, false)
	if err != nil {
		return err
	}
	if !NotificationEnabled {
		return nil
	}
	for i := 0; i < 3; i++ {
		err = CurrentProvider().SendTextMessage(message, title)
		if err == nil {
			auditlog.Log("", "", "Message sent: "+title, "info")
			return nil
		}
	}
	auditlog.Log("", "", "Failed to send message after 3 attempts: "+err.Error()+","+title, "error")
	return err
}
func SendEvent(event models.EventMessage) error {
	if CurrentProvider() == nil {
		return fmt.Errorf("message sender provider is not initialized")
	}
	if event.Time.IsZero() {
		event.Time = time.Now().UTC()
	} else {
		event.Time = event.Time.UTC()
	}
	var err error
	cfg, err := config.GetMany(map[string]any{
		config.NotificationEnabledKey:  false,
		config.NotificationTemplateKey: "{{emoji}}{{emoji}}{{emoji}}\nEvent: {{event}}\nClients: {{client}}\nMessage: {{message}}\nTime: {{time}}",
	})
	if err != nil {
		return err
	}
	if !cfg[config.NotificationEnabledKey].(bool) {
		return nil
	}

	// Check if the provider implements the IEventMessageSender interface
	if eventSender, ok := CurrentProvider().(factory.IEventMessageSender); ok {
		// If implemented, call SendEvent directly
		for i := 0; i < 3; i++ {
			err = eventSender.SendEvent(event)
			if err == nil || err.Error() == "short response: \x00\x00\x00\x1a\x00\x00\x00" {
				auditlog.Log("", "", "Event message sent: "+event.Event, "info")
				return nil
			}
		}
		auditlog.Log("", "", "Failed to send event message after 3 attempts: "+err.Error()+","+event.Event, "error")
		return err
	}

	// If not implemented, use templates to format text messages
	messageTemplate := cfg[config.NotificationTemplateKey].(string)

	messageTemplate = parseTemplate(messageTemplate, event)

	for i := 0; i < 3; i++ {
		err = CurrentProvider().SendTextMessage(messageTemplate, event.Event)
		if err == nil || err.Error() == "short response: \x00\x00\x00\x1a\x00\x00\x00" { // QQ will return this error, but in fact the message is sent successfully
			auditlog.Log("", "", "Event message sent: "+event.Event, "info")
			return nil
		}
	}
	auditlog.Log("", "", "Failed to send event message after 3 attempts: "+err.Error()+","+event.Event, "error")
	return err
}

func parseTemplate(messageTemplate string, event models.EventMessage) string {
	// Aggregate client names. If Name is empty, fall back to UUID.
	clientNames := make([]string, 0, len(event.Clients))
	for _, c := range event.Clients {
		name := c.Name
		if strings.TrimSpace(name) == "" {
			// fallback to UUID when name is not set
			name = c.UUID
		}
		clientNames = append(clientNames, name)
	}
	joinedClients := strings.Join(clientNames, ", ")

	replaceMap := map[string]string{
		"{{event}}":   event.Event,
		"{{client}}":  joinedClients,
		"{{time}}":    event.Time.UTC().Format(time.RFC3339Nano),
		"{{message}}": event.Message,
		"{{emoji}}":   event.Emoji,
	}
	result := messageTemplate
	for placeholder, value := range replaceMap {
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}
