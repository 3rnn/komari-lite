package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/komari-monitor/komari/utils/messageSender/factory"
	"github.com/komari-monitor/komari/utils/messageSender/outboundhttp"
)

type WebhookSender struct {
	Addition
}

func (w *WebhookSender) GetName() string {
	return "webhook"
}

func (w *WebhookSender) GetConfiguration() factory.Configuration {
	return &w.Addition
}

func (w *WebhookSender) Init() error {
	return nil
}

func (w *WebhookSender) Destroy() error {
	return nil
}

func (w *WebhookSender) SendTextMessage(message, title string) error {
	if w.Addition.URL == "" {
		return fmt.Errorf("webhook URL is not configured")
	}

	method := strings.ToUpper(w.Addition.Method)
	if method == "" {
		method = "GET" // Use GET by default
	}

	client := outboundhttp.NewClient(30 * time.Second)

	var req *http.Request
	var err error

	switch method {
	case "POST":
		req, err = w.createPOSTRequest(message, title)
	case "GET":
		req, err = w.createGETRequest(message, title)
	default:
		return fmt.Errorf("unsupported HTTP method: %s", method)
	}

	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	// Parse and set custom headers
	if w.Addition.Headers != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(w.Addition.Headers), &headers); err == nil {
			for key, value := range headers {
				req.Header.Set(key, value)
			}
		}
	}

	// Set up basic authentication
	if w.Addition.Username != "" && w.Addition.Password != "" {
		req.SetBasicAuth(w.Addition.Username, w.Addition.Password)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (w *WebhookSender) createPOSTRequest(message, title string) (*http.Request, error) {
	contentType := w.Addition.ContentType
	if contentType == "" {
		contentType = "application/json"
	}

	// User-defined template, determine how to replace placeholders according to Content-Type
	var body string
	if isJSONContentType(contentType) {
		body = w.replaceTemplateJSON(w.Addition.Body, message, title)
	} else {
		body = w.replaceTemplate(w.Addition.Body, message, title)
	}

	req, err := http.NewRequest("POST", w.Addition.URL, bytes.NewBufferString(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", contentType)
	return req, nil
}

func (w *WebhookSender) createGETRequest(message, title string) (*http.Request, error) {
	URL := w.replaceTemplate(w.Addition.URL, message, title)
	u, err := url.Parse(URL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %v", err)
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	return req, nil
}

// replaceTemplate replaces the {{message}} and {{title}} placeholders in the template (without escaping)
func (w *WebhookSender) replaceTemplate(template, message, title string) string {
	result := template
	result = strings.ReplaceAll(result, "{{message}}", message)
	result = strings.ReplaceAll(result, "{{title}}", title)
	return result
}

// replaceTemplateJSON replaces placeholders in JSON scenarios and uses \uXXXX forms for escaping (especially newlines/control characters)
func (w *WebhookSender) replaceTemplateJSON(template, message, title string) string {
	result := template
	result = strings.ReplaceAll(result, "{{message}}", jsonUnicodeEscapeString(message))
	result = strings.ReplaceAll(result, "{{title}}", jsonUnicodeEscapeString(title))
	return result
}

// isJSONContentType determines whether it is a JSON content type
func isJSONContentType(ct string) bool {
	return strings.Contains(strings.ToLower(ct), "application/json")
}

// jsonUnicodeEscapeString escapes the string according to JSON rules, and try to use \uXXXX forms (especially control characters, quotes and backslashes)
// Note: The result fits within quotes of a JSON string literal.
func jsonUnicodeEscapeString(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString("\\u0022")
		case '\\':
			b.WriteString("\\u005C")
		case '\b':
			b.WriteString("\\u0008")
		case '\f':
			b.WriteString("\\u000C")
		case '\n':
			b.WriteString("\\u000A")
		case '\r':
			b.WriteString("\\u000D")
		case '\t':
			b.WriteString("\\u0009")
		default:
			if r < 0x20 {
				// Other control characters
				b.WriteString(fmt.Sprintf("\\u%04X", r))
			} else {
				// The remaining characters are output as-is (UTF-8), JSON allows non-ASCII characters
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}
