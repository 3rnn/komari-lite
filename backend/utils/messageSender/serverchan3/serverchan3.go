package serverchan3

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

// ServerChan3Sender is implemented for Serverchan³ push
type ServerChan3Sender struct {
	Addition
}

// GetName returns the push channel name
func (s *ServerChan3Sender) GetName() string {
	return "Server\u9171³"
}

// GetConfiguration returns the configuration structure pointer
func (s *ServerChan3Sender) GetConfiguration() factory.Configuration {
	return &s.Addition
}

// Init initialization (no need to process currently)
func (s *ServerChan3Sender) Init() error { return nil }

// Destroy cleanup (no need to process currently)
func (s *ServerChan3Sender) Destroy() error { return nil }

// SendTextMessage Sends a text message (with headers).
// Simplified to a fixed JSON POST: only {"title": ..., "desp": ...} is assembled internally, and users are not allowed to customize the request body or content type.
func (s *ServerChan3Sender) SendTextMessage(message, title string) error {
	apiURL := strings.TrimSpace(s.Addition.APIURL)
	if apiURL == "" {
		return fmt.Errorf("ServerChan3 API URL (api_url) is not configured")
	}

	// Simplified verification: If the title and body are both empty, refuse to send
	finalTitle := strings.TrimSpace(title)
	finalMessage := strings.TrimSpace(message)
	if finalTitle == "" && finalMessage == "" {
		return fmt.Errorf("serverchan3: both title and body are empty")
	}

	// Fixed JSON payload, including title, desp, and supports optional tags (split by |)
	payload := map[string]string{
		"title": finalTitle,
		"desp":  finalMessage,
	}
	if tagsStr := strings.TrimSpace(s.Addition.Tags); tagsStr != "" {
		parts := strings.Split(tagsStr, "|")
		var cleaned []string
		for _, p := range parts {
			t := strings.TrimSpace(p)
			if t != "" {
				cleaned = append(cleaned, t)
			}
		}
		if len(cleaned) > 0 {
			// ServerChan3 requires a string for its verification prompt; join tags with |.
			payload["tags"] = strings.Join(cleaned, "|")
		}
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to build JSON: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	// Fixed setting Content-Type
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Read the response body for error message printing
		b, _ := io.ReadAll(resp.Body)
		msg := strings.TrimSpace(string(b))
		if msg == "" {
			return fmt.Errorf("API returned a non-2xx status: %d", resp.StatusCode)
		}
		return fmt.Errorf("API returned a non-2xx status: %d, response: %s", resp.StatusCode, msg)
	}

	return nil
}

// The previous templates, form parsing and manual escaping tools are no longer needed, simplifying implementation.

// Make sure to implement the IMessageSender interface
var _ factory.IMessageSender = (*ServerChan3Sender)(nil)
