package models

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

const (
	ThemeConfigurationManaged  = "managed"
	ThemeConfigurationRaw      = "raw"
	ThemeConfigurationRedirect = "redirect"
)

// Theme represents a komari theme information
type Theme struct {
	Name          any           `json:"name"`          // Theme name, supports string or multilingual object
	Short         string        `json:"short"`         // Short name, used as folder name
	Description   any           `json:"description"`   // Topic description, supports string or multilingual object
	Version       string        `json:"version"`       // version number
	Author        any           `json:"author"`        // Author, supports string or multilingual objects
	URL           string        `json:"url"`           // Topic URL
	Preview       string        `json:"preview"`       // Preview image relative path
	Configuration Configuration `json:"configuration"` // Declare configuration items
}

type Configuration struct {
	Type string `json:"type"` // managed raw redirect
	Icon string `json:"icon"` // icon
	Name any    `json:"name"`
	Data any    `json:"data"` // Configuration data
}

type ManagedThemeConfigurationItem struct {
	Key      string `json:"key"`
	Name     any    `json:"name"`
	Required bool   `json:"required"`
	Type     string `json:"type"` // string number select switch title richtext nodes pingtasks
	Options  string `json:"options"`
	Default  any    `json:"default"`
	Help     any    `json:"help"`
}

// IsLocalizedText reports whether a manifest text field contains at least one
// non-empty string in either the legacy scalar or localized-object form.
func IsLocalizedText(value any) bool {
	switch text := value.(type) {
	case string:
		return strings.TrimSpace(text) != ""
	case map[string]any:
		for _, item := range text {
			if itemText, ok := item.(string); ok && strings.TrimSpace(itemText) != "" {
				return true
			}
		}
	case map[string]string:
		for _, item := range text {
			if strings.TrimSpace(item) != "" {
				return true
			}
		}
	}
	return false
}

type ThemeConfiguration struct {
	Short string `json:"short" gorm:"primaryKey;unique;not null"`
	Data  string `json:"data" gorm:"type:longtext" default:"{}"`
}

func (t Theme) ConfigurationType() string {
	typ := strings.ToLower(strings.TrimSpace(t.Configuration.Type))
	if typ == "" {
		return ThemeConfigurationManaged
	}
	return typ
}

func (t Theme) RawHTML() (string, bool) {
	if t.ConfigurationType() != ThemeConfigurationRaw {
		return "", false
	}
	return configurationDataString(t.Configuration.Data)
}

func (t Theme) RedirectTarget() (string, bool) {
	if t.ConfigurationType() != ThemeConfigurationRedirect {
		return "", false
	}
	return NormalizeThemeRedirectTarget(t.Configuration.Data)
}

func (t Theme) ValidateConfiguration() error {
	switch t.ConfigurationType() {
	case ThemeConfigurationManaged:
		return nil
	case ThemeConfigurationRaw:
		html, ok := t.RawHTML()
		if !ok || strings.TrimSpace(html) == "" {
			return fmt.Errorf("Raw themes require an HTML string in configuration.data")
		}
		return nil
	case ThemeConfigurationRedirect:
		if _, ok := t.RedirectTarget(); !ok {
			return fmt.Errorf("Redirect themes require a site-relative path in configuration.data")
		}
		return nil
	default:
		return fmt.Errorf("Unsupported theme type: %s", t.Configuration.Type)
	}
}

func NormalizeThemeRedirectTarget(data any) (string, bool) {
	target, ok := configurationDataString(data)
	if !ok {
		return "", false
	}

	target = strings.TrimSpace(target)
	if target == "" || strings.Contains(target, "\\") || strings.HasPrefix(target, "//") {
		return "", false
	}

	parsed, err := url.Parse(target)
	if err != nil || parsed.IsAbs() || parsed.Host != "" {
		return "", false
	}

	cleanInputPath := parsed.Path
	if strings.HasPrefix(cleanInputPath, "/") {
		cleanInputPath = strings.TrimLeft(cleanInputPath, "/")
	} else {
		for strings.HasPrefix(cleanInputPath, "../") {
			cleanInputPath = strings.TrimPrefix(cleanInputPath, "../")
		}
	}

	for _, segment := range strings.Split(cleanInputPath, "/") {
		if segment == ".." {
			return "", false
		}
	}

	cleanPath := cleanInputPath
	if cleanPath == "" {
		cleanPath = "/"
	} else {
		cleanPath = path.Clean(cleanPath)
		if cleanPath == "." {
			cleanPath = "/"
		} else {
			cleanPath = "/" + strings.TrimPrefix(cleanPath, "/")
		}
	}

	normalized := url.URL{
		Path:     cleanPath,
		RawQuery: parsed.RawQuery,
		Fragment: parsed.Fragment,
	}
	return normalized.String(), true
}

func configurationDataString(data any) (string, bool) {
	value, ok := data.(string)
	return value, ok
}
