package serverchanturbo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/komari-monitor/komari/utils/messageSender/factory"
)

// ServerChanTurboSender implements Serverchan Turbo push
type ServerChanTurboSender struct {
	Addition
}

// GetName returns the push channel name
func (s *ServerChanTurboSender) GetName() string {
	return "Server\u9171Turbo"
}

// GetConfiguration returns the configuration structure pointer
func (s *ServerChanTurboSender) GetConfiguration() factory.Configuration {
	return &s.Addition
}

// Init initialization (no need to process currently)
func (s *ServerChanTurboSender) Init() error { return nil }

// Destroy cleanup (no need to process currently)
func (s *ServerChanTurboSender) Destroy() error { return nil }

// SendTextMessage is always sent as JSON, only assembles title and desp, and supports optional channel/noip/openid
func (s *ServerChanTurboSender) SendTextMessage(message, title string) error {
	apiURL := strings.TrimSpace(s.Addition.APIURL)
	if apiURL == "" {
		return fmt.Errorf("ServerChan Turbo API URL (api_url) is not configured")
	}

	finalTitle := strings.TrimSpace(title)
	finalMessage := strings.TrimSpace(message)
	if finalTitle == "" && finalMessage == "" {
		return fmt.Errorf("serverchanturbo: both title and body are empty")
	}

	payload := map[string]interface{}{
		"title": finalTitle,
		"desp":  finalMessage,
	}
	if ch := strings.TrimSpace(s.Addition.Channel); ch != "" {
		payload["channel"] = ch
	}
	if noip := strings.TrimSpace(s.Addition.NoIP); noip == "1" {
		payload["noip"] = "1"
	}
	if openid := strings.TrimSpace(s.Addition.OpenID); openid != "" {
		payload["openid"] = openid
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("serverchanturbo: failed to build JSON: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return fmt.Errorf("serverchanturbo: failed to create request: %v", err)
	}
	// To use JSON to pass parameters, you need to set Content-Type
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("serverchanturbo: failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		msg := strings.TrimSpace(string(b))
		if msg == "" {
			return fmt.Errorf("serverchanturbo: API returned a non-2xx status: %d", resp.StatusCode)
		}
		return fmt.Errorf("serverchanturbo: API returned a non-2xx status: %d, response: %s", resp.StatusCode, msg)
	}

	return nil
}

// Make sure to implement the IMessageSender interface
var _ factory.IMessageSender = (*ServerChanTurboSender)(nil)
