package javascript

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/utils/messageSender/factory"
)

type JavaScriptSender struct {
	Addition
	vm          *goja.Runtime
	noopProgram *goja.Program
}

func (j *JavaScriptSender) GetName() string {
	return "Javascript"
}

func (j *JavaScriptSender) GetConfiguration() factory.Configuration {
	return &j.Addition
}

func (j *JavaScriptSender) Init() error {
	if j.Addition.Script == "" {
		return errors.New("JavaScript script is empty")
	}

	// Create the JavaScript runtime
	j.vm = goja.New()

	// Precompile a no-op program to drive the microtask queue
	prog, errc := goja.Compile("noop.js", "void 0", false)
	if errc == nil {
		j.noopProgram = prog
	}

	// Set require support
	new(require.Registry).Enable(j.vm)

	// Inject global objects and functions
	j.setupGlobals()

	// Load user script
	_, err := j.vm.RunString(j.Addition.Script)
	if err != nil {
		return fmt.Errorf("failed to load JavaScript script: %v", err)
	}

	// Verify that the sendMessage function exists
	sendMessage := j.vm.Get("sendMessage")
	if sendMessage == nil || goja.IsUndefined(sendMessage) {
		return errors.New("sendMessage function not defined in script")
	}

	// Verify if callable
	if _, ok := goja.AssertFunction(sendMessage); !ok {
		return errors.New("sendMessage is not a function")
	}

	// The sendEvent function is optional and is not required to exist.

	return nil
}

func (j *JavaScriptSender) Destroy() error {
	if j.vm != nil {
		j.vm = nil
	}
	return nil
}

func (j *JavaScriptSender) SendTextMessage(message, title string) error {
	if j.vm == nil {
		if err := j.Init(); err != nil {
			return err
		}
	}

	// Get sendMessage function
	sendMessageFunc, ok := goja.AssertFunction(j.vm.Get("sendMessage"))
	if !ok {
		return errors.New("sendMessage is not a callable function")
	}

	// Call sendMessage function
	resultChan := make(chan error, 1)
	timeoutChan := time.After(30 * time.Second)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				resultChan <- fmt.Errorf("JavaScript panic: %v", r)
			}
		}()

		result, err := sendMessageFunc(goja.Undefined(), j.vm.ToValue(message), j.vm.ToValue(title))
		if err != nil {
			resultChan <- fmt.Errorf("JavaScript error: %v", err)
			return
		}

		// Handling Promise return values
		if promise, ok := result.Export().(*goja.Promise); ok {
			// Wait for the Promise to complete
			ticker := time.NewTicker(50 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-timeoutChan:
					resultChan <- errors.New("JavaScript execution timeout")
					return
				case <-ticker.C:
					// Run microtasks to process Promise callbacks
					j.runMicrotasks()

					state := promise.State()
					if state == goja.PromiseStateFulfilled {
						// Promise completed successfully, check the return value
						promiseResult := promise.Result()
						if !promiseResult.ToBoolean() {
							resultChan <- errors.New("sendMessage returned false")
						} else {
							resultChan <- nil
						}
						return
					} else if state == goja.PromiseStateRejected {
						// Promise rejected
						resultChan <- fmt.Errorf("Promise rejected: %v", promise.Result())
						return
					}
					// state == goja.PromiseStatePending, continue to wait
				}
			}
		} else {
			// Handle boolean and other return values
			if result.ToBoolean() {
				resultChan <- nil
			} else {
				resultChan <- errors.New("sendMessage returned false")
			}
		}
	}()

	select {
	case err := <-resultChan:
		return err
	case <-timeoutChan:
		return errors.New("JavaScript execution timeout after 30 seconds")
	}
}

func (j *JavaScriptSender) SendEvent(event models.EventMessage) error {
	if j.vm == nil {
		if err := j.Init(); err != nil {
			return err
		}
	}

	// Check if sendEvent function is defined
	sendEventValue := j.vm.Get("sendEvent")
	if sendEventValue == nil || goja.IsUndefined(sendEventValue) {
		// If sendEvent is not defined, fallback to using SendTextMessage
		return j.fallbackToTextMessage(event)
	}

	sendEventFunc, ok := goja.AssertFunction(sendEventValue)
	if !ok {
		// If sendEvent is not a function, fallback to SendTextMessage
		return j.fallbackToTextMessage(event)
	}

	// Convert EventMessage to a JavaScript object
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %v", err)
	}

	var eventMap map[string]interface{}
	if err := json.Unmarshal(eventJSON, &eventMap); err != nil {
		return fmt.Errorf("failed to unmarshal event: %v", err)
	}

	// Call sendEvent function
	resultChan := make(chan error, 1)
	timeoutChan := time.After(30 * time.Second)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				resultChan <- fmt.Errorf("JavaScript panic: %v", r)
			}
		}()

		result, err := sendEventFunc(goja.Undefined(), j.vm.ToValue(eventMap))
		if err != nil {
			resultChan <- fmt.Errorf("JavaScript error: %v", err)
			return
		}

		// Handling Promise return values
		if promise, ok := result.Export().(*goja.Promise); ok {
			// Wait for the Promise to complete
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-timeoutChan:
					resultChan <- errors.New("JavaScript execution timeout")
					return
				case <-ticker.C:
					// Run microtasks to process Promise callbacks
					j.runMicrotasks()

					state := promise.State()
					if state == goja.PromiseStateFulfilled {
						// Promise completed successfully, check the return value
						promiseResult := promise.Result()
						if !promiseResult.ToBoolean() {
							resultChan <- errors.New("sendEvent returned false")
						} else {
							resultChan <- nil
						}
						return
					} else if state == goja.PromiseStateRejected {
						// Promise rejected
						resultChan <- fmt.Errorf("Promise rejected: %v", promise.Result())
						return
					}
					// state == goja.PromiseStatePending, continue to wait
				}
			}
		} else {
			// Handle boolean and other return values
			if result.ToBoolean() {
				resultChan <- nil
			} else {
				resultChan <- errors.New("sendEvent returned false")
			}
		}
	}()

	select {
	case err := <-resultChan:
		return err
	case <-timeoutChan:
		return errors.New("JavaScript execution timeout after 30 seconds")
	}
}

// fallbackToTextMessage When sendEvent is not defined, fallback to using the text message format
func (j *JavaScriptSender) fallbackToTextMessage(event models.EventMessage) error {
	// Build a simple text message
	message := fmt.Sprintf("%s%s%s\nEvent: %s\nMessage: %s\nTime: %s",
		event.Emoji, event.Emoji, event.Emoji,
		event.Event,
		event.Message,
		event.Time.UTC().Format(time.RFC3339Nano))

	// Add client information
	if len(event.Clients) > 0 {
		clientNames := make([]string, 0, len(event.Clients))
		for _, c := range event.Clients {
			name := c.Name
			if name == "" {
				name = c.UUID
			}
			clientNames = append(clientNames, name)
		}
		message = fmt.Sprintf("%s%s%s\nEvent: %s\nClients: %s\nMessage: %s\nTime: %s",
			event.Emoji, event.Emoji, event.Emoji,
			event.Event,
			clientNames,
			event.Message,
			event.Time.UTC().Format(time.RFC3339Nano))
	}

	return j.SendTextMessage(message, event.Event)
}

// runMicrotasks safely pushes Goja's microtask queue (e.g. Promise callbacks)
func (j *JavaScriptSender) runMicrotasks() {
	if j.vm == nil {
		return
	}
	if j.noopProgram != nil {
		_, _ = j.vm.RunProgram(j.noopProgram)
		return
	}
	// Fallback: run a no-op program directly
	_, _ = j.vm.RunString("void 0")
}

func (j *JavaScriptSender) setupGlobals() {
	// Inject console.log
	console := j.vm.NewObject()
	console.Set("log", func(call goja.FunctionCall) goja.Value {
		var args []interface{}
		for _, arg := range call.Arguments {
			args = append(args, arg.Export())
		}
		fmt.Println(args...)
		return goja.Undefined()
	})
	console.Set("error", func(call goja.FunctionCall) goja.Value {
		fmt.Print("Error: ")
		for i, arg := range call.Arguments {
			if i > 0 {
				fmt.Print(" ")
			}
			fmt.Print(arg.Export())
		}
		fmt.Println()
		return goja.Undefined()
	})
	j.vm.Set("console", console)

	// Inject fetch API
	j.vm.Set("fetch", j.createFetchFunction())

	// Inject XMLHttpRequest (xhr)
	j.vm.Set("XMLHttpRequest", j.createXHRConstructor())

	// Inject setTimeout
	j.vm.Set("setTimeout", func(call goja.FunctionCall) goja.Value {
		callback := call.Argument(0)
		delay := call.Argument(1).ToInteger()

		go func() {
			time.Sleep(time.Duration(delay) * time.Millisecond)
			if fn, ok := goja.AssertFunction(callback); ok {
				fn(goja.Undefined())
			}
		}()

		return goja.Undefined()
	})

	// Inject Promise constructor
	j.vm.RunString(`
		if (typeof Promise === 'undefined') {
			// The Promise polyfill is provided automatically by goja
		}
	`)
}

func init() {
	factory.RegisterMessageSender(func() factory.IMessageSender {
		return &JavaScriptSender{}
	})
}

// Make sure you implement the IMessageSender interface
var _ factory.IMessageSender = (*JavaScriptSender)(nil)
