package rpc

// meta.go
// Maintains optional help/description information for registered RPC methods for use by rpc.help.
// Does not affect the original Register behavior; when metadata is not explicitly attached, a placeholder containing only the name will be automatically generated.

import (
	"sort"
	"strings"
	"sync"
)

// ParamMeta describes a single parameter.
type ParamMeta struct {
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Description string `json:"description,omitempty"`
}

// MethodMeta Help information describing the method.
type MethodMeta struct {
	Name        string      `json:"name"`
	Summary     string      `json:"summary,omitempty"`
	Description string      `json:"description,omitempty"`
	Params      []ParamMeta `json:"params,omitempty"`
	Returns     string      `json:"returns,omitempty"`
	Example     any         `json:"example,omitempty"`
}

var (
	muMetas     sync.RWMutex
	methodMetas = map[string]*MethodMeta{}
)

// getMetaUnsafe internal use: the caller needs to hold the read/write lock of handlers or synchronize itself (independent lock here guarantees concurrency).
func getMetaUnsafe(name string) *MethodMeta {
	muMetas.RLock()
	m := methodMetas[name]
	muMetas.RUnlock()
	return m
}

// ensureMeta ensures basic metadata (minimum Name) is present.
func ensureMeta(name string) {
	if name == "" {
		return
	}
	muMetas.Lock()
	if _, ok := methodMetas[name]; !ok {
		methodMetas[name] = &MethodMeta{Name: name}
	}
	muMetas.Unlock()
}

// RegisterMeta appends/overwrites metadata for registered methods (the Name field is automatically filled in if it is empty).
func RegisterMeta(name string, meta *MethodMeta) {
	if name == "" || meta == nil {
		return
	}
	if meta.Name == "" {
		meta.Name = name
	}
	muMetas.Lock()
	methodMetas[name] = meta
	muMetas.Unlock()
}

// RegisterWithMeta registers methods and metadata at the same time; if registration fails, an error is returned.
func RegisterWithMeta(method string, h Handler, meta *MethodMeta) error {
	if err := Register(method, h); err != nil {
		return err
	}
	if meta != nil {
		RegisterMeta(method, meta)
	} else {
		ensureMeta(method)
	}
	return nil
}

// listMetas Gets a copy of the brief metadata for all methods (by the given filter).
func listMetas(includeInternal bool) []*MethodMeta {
	muHandlers.RLock()
	names := make([]string, 0, len(handlers))
	for n := range handlers {
		names = append(names, n)
	}
	muHandlers.RUnlock()
	out := make([]*MethodMeta, 0, len(names))
	for _, n := range names {
		if !includeInternal && strings.HasPrefix(n, "rpc.") {
			continue
		}
		if m := getMetaUnsafe(n); m != nil {
			out = append(out, &MethodMeta{ // Copy brief fields
				Name:    m.Name,
				Summary: m.Summary,
			})
		} else {
			out = append(out, &MethodMeta{Name: n})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
