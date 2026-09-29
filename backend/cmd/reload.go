package cmd

import (
	logger "github.com/komari-monitor/komari/utils/log"

	"github.com/komari-monitor/komari/pkg/config"
)

// reloadHandler handles one configuration hot-reload event.
//
// name is used only for logging; each handler decides which changed keys matter.
// Handlers are isolated so a panic in one does not affect the others.
type reloadHandler struct {
	name    string
	handler func(config.ConfigEvent)
}

// ReloadManager centralizes configuration hot-reload handlers.
//
// Previously, config.Subscribe calls were scattered across cmd/server.go, cors.go, and elsewhere:
//   - it was unclear which components listened for which keys;
//   - one handler's panic could interrupt other handlers in the same callback;
//   - subscriptions could not be stopped together during shutdown.
//
// ReloadManager registers handlers in one place and dispatches events via one config.Subscribe call,
// isolating panics in each handler.
type ReloadManager struct {
	handlers []reloadHandler
	started  bool
}

// NewReloadManager creates an empty hot-reload manager.
func NewReloadManager() *ReloadManager {
	return &ReloadManager{}
}

// Register adds a named hot-reload handler. Call it before Start.
func (m *ReloadManager) Register(name string, handler func(config.ConfigEvent)) {
	if handler == nil {
		return
	}
	m.handlers = append(m.handlers, reloadHandler{name: name, handler: handler})
}

// Start subscribes to config once, after which each event is distributed to all registered handlers.
// Repeated calls have no side effects.
func (m *ReloadManager) Start() {
	if m.started {
		return
	}
	m.started = true
	config.Subscribe(func(event config.ConfigEvent) {
		for _, h := range m.handlers {
			m.dispatch(h, event)
		}
	})
}

// dispatch runs a handler with panic isolation.
func (m *ReloadManager) dispatch(h reloadHandler, event config.ConfigEvent) {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("reload", "config reload handler %q panicked: %v", h.name, r)
		}
	}()
	h.handler(event)
}
