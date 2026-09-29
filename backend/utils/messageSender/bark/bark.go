package bark

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/komari-monitor/komari/utils/messageSender/factory"
)

type BarkSender struct {
	Addition
}

func (b *BarkSender) GetName() string {
	return "bark"
}

func (b *BarkSender) GetConfiguration() factory.Configuration {
	return &b.Addition
}

func (b *BarkSender) Init() error {
	// Verify ServerURL format
	if b.Addition.ServerURL != "" {
		if _, err := url.Parse(b.Addition.ServerURL); err != nil {
			return fmt.Errorf("invalid server URL: %v", err)
		}
	}
	return nil
}

func (b *BarkSender) Destroy() error {
	return nil
}

func (b *BarkSender) SendTextMessage(message, title string) error {
	if b.Addition.DeviceKey == "" {
		return fmt.Errorf("device key is required")
	}

	if message == "" {
		return fmt.Errorf("message is empty")
	}

	// Prepare request data
	payload := map[string]interface{}{
		"body":       message,
		"device_key": b.Addition.DeviceKey,
	}

	// If there is a title, add the title
	if title != "" {
		payload["title"] = title
	}

	// Add optional parameters
	if b.Addition.Icon != "" {
		payload["icon"] = b.Addition.Icon
	}

	if b.Addition.Level != "" {
		payload["level"] = b.Addition.Level
	}

	// Serialize JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %v", err)
	}

	// Build request URL
	serverURL := b.Addition.ServerURL
	if serverURL == "" {
		serverURL = "https://api.day.app"
	}

	// Ensure the URL has the correct suffix
	serverURL = strings.TrimRight(serverURL, "/")
	requestURL := fmt.Sprintf("%s/push", serverURL)

	// Send an HTTP POST request
	resp, err := http.Post(requestURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bark API returned non-OK status: %d", resp.StatusCode)
	}

	// Parse the response
	var result struct {
		Code    int         `json:"code"`
		Message string      `json:"message"`
		Data    interface{} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		// If parsing fails, the transmission is considered successful (some Bark servers may return plain text)
		return nil
	}

	// Check the Bark API response
	if result.Code != 200 {
		return fmt.Errorf("bark API error (code: %d): %s", result.Code, result.Message)
	}

	return nil
}

// Make sure you implement the IMessageSender interface
var _ factory.IMessageSender = (*BarkSender)(nil)
