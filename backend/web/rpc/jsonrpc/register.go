package jsonrpc

import "github.com/komari-monitor/komari/pkg/rpc"

// Register registers the method in the default group "common".
func Register(name string, cb rpc.Handler) error {
	return RegisterWithGroupAndMeta(name, "common", cb, &rpc.MethodMeta{
		Name:        name,
		Summary:     "This method does not provide a summary",
		Description: "This method does not provide a description",
	})
}

// RegisterWithGroupAndMeta registers a callback as "group:name" with metadata.
// use the default group "common" when group is empty.
func RegisterWithGroupAndMeta(name, group string, cb rpc.Handler, meta *rpc.MethodMeta) error {
	if group == "" {
		group = "common"
	}
	return rpc.RegisterWithMeta(group+":"+name, cb, meta)
}
