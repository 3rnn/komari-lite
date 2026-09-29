package javascript

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/komari-monitor/komari/utils/messageSender/outboundhttp"
)

// createFetchFunction creates a fetch API implementation
func (j *JavaScriptSender) createFetchFunction() func(goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			panic(j.vm.NewTypeError("fetch requires at least 1 argument"))
		}

		url := call.Argument(0).String()

		// parsing options
		options := map[string]interface{}{
			"method":  "GET",
			"headers": make(map[string]string),
			"body":    "",
		}

		if len(call.Arguments) > 1 {
			optObj := call.Argument(1).ToObject(j.vm)
			if optObj != nil {
				if method := optObj.Get("method"); method != nil && !goja.IsUndefined(method) {
					options["method"] = method.String()
				}
				if headers := optObj.Get("headers"); headers != nil && !goja.IsUndefined(headers) {
					headersObj := headers.ToObject(j.vm)
					if headersObj != nil {
						headerMap := make(map[string]string)
						for _, key := range headersObj.Keys() {
							headerMap[key] = headersObj.Get(key).String()
						}
						options["headers"] = headerMap
					}
				}
				if body := optObj.Get("body"); body != nil && !goja.IsUndefined(body) {
					options["body"] = body.String()
				}
			}
		}

		// Create a Promise
		promise, resolve, reject := j.vm.NewPromise()

		go func() {
			defer func() {
				if r := recover(); r != nil {
					reject(j.vm.ToValue(fmt.Sprintf("fetch panic: %v", r)))
				}
			}()

			// Create HTTP request
			method := options["method"].(string)
			var body io.Reader
			if options["body"].(string) != "" {
				body = strings.NewReader(options["body"].(string))
			}

			req, err := http.NewRequest(method, url, body)
			if err != nil {
				reject(j.vm.ToValue(fmt.Sprintf("Failed to create request: %v", err)))
				return
			}

			// Set request header
			headers := options["headers"].(map[string]string)
			for key, value := range headers {
				req.Header.Set(key, value)
			}

			// Send request
			client := outboundhttp.NewClient(30 * time.Second)
			resp, err := client.Do(req)
			if err != nil {
				reject(j.vm.ToValue(fmt.Sprintf("Fetch failed: %v", err)))
				return
			}
			defer resp.Body.Close()

			// Read response body
			bodyBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				reject(j.vm.ToValue(fmt.Sprintf("Failed to read response: %v", err)))
				return
			}

			// Create the response object
			responseObj := j.vm.NewObject()
			responseObj.Set("status", resp.StatusCode)
			responseObj.Set("statusText", resp.Status)
			responseObj.Set("ok", resp.StatusCode >= 200 && resp.StatusCode < 300)

			// response header
			headersObj := j.vm.NewObject()
			for key, values := range resp.Header {
				if len(values) > 0 {
					headersObj.Set(key, values[0])
				}
			}
			responseObj.Set("headers", headersObj)

			// text() method
			responseObj.Set("text", func(goja.FunctionCall) goja.Value {
				textPromise, textResolve, _ := j.vm.NewPromise()
				textResolve(j.vm.ToValue(string(bodyBytes)))
				return j.vm.ToValue(textPromise)
			})

			// json() method
			responseObj.Set("json", func(goja.FunctionCall) goja.Value {
				jsonPromise, jsonResolve, jsonReject := j.vm.NewPromise()
				var result interface{}
				if err := json.Unmarshal(bodyBytes, &result); err != nil {
					jsonReject(j.vm.ToValue(fmt.Sprintf("Failed to parse JSON: %v", err)))
				} else {
					jsonResolve(j.vm.ToValue(result))
				}
				return j.vm.ToValue(jsonPromise)
			})

			resolve(responseObj)
		}()

		return j.vm.ToValue(promise)
	}
}

// createXHRConstructor creates an XMLHttpRequest constructor
func (j *JavaScriptSender) createXHRConstructor() func(goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		xhr := call.This

		// Internal state
		var method, url string
		var headers = make(map[string]string)
		var requestBody string
		var async = true

		// readyState
		xhr.Set("readyState", 0)
		xhr.Set("status", 0)
		xhr.Set("statusText", "")
		xhr.Set("responseText", "")
		xhr.Set("response", "")

		// event handler
		xhr.Set("onreadystatechange", goja.Null())
		xhr.Set("onload", goja.Null())
		xhr.Set("onerror", goja.Null())

		// open method
		xhr.Set("open", func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) < 2 {
				panic(j.vm.NewTypeError("open requires at least 2 arguments"))
			}
			method = call.Argument(0).String()
			url = call.Argument(1).String()
			if len(call.Arguments) > 2 {
				async = call.Argument(2).ToBoolean()
			}
			xhr.Set("readyState", 1)
			j.callHandler(xhr, "onreadystatechange")
			return goja.Undefined()
		})

		// setRequestHeader method
		xhr.Set("setRequestHeader", func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) < 2 {
				panic(j.vm.NewTypeError("setRequestHeader requires 2 arguments"))
			}
			key := call.Argument(0).String()
			value := call.Argument(1).String()
			headers[key] = value
			return goja.Undefined()
		})

		// send method
		xhr.Set("send", func(call goja.FunctionCall) goja.Value {
			if len(call.Arguments) > 0 && !goja.IsUndefined(call.Argument(0)) && !goja.IsNull(call.Argument(0)) {
				requestBody = call.Argument(0).String()
			}

			sendFunc := func() {
				defer func() {
					if r := recover(); r != nil {
						xhr.Set("readyState", 4)
						xhr.Set("status", 0)
						xhr.Set("statusText", fmt.Sprintf("Error: %v", r))
						j.callHandler(xhr, "onerror")
						j.callHandler(xhr, "onreadystatechange")
					}
				}()

				// Create request
				var body io.Reader
				if requestBody != "" {
					body = bytes.NewReader([]byte(requestBody))
				}

				req, err := http.NewRequest(method, url, body)
				if err != nil {
					xhr.Set("readyState", 4)
					xhr.Set("status", 0)
					xhr.Set("statusText", err.Error())
					j.callHandler(xhr, "onerror")
					j.callHandler(xhr, "onreadystatechange")
					return
				}

				// Set request header
				for key, value := range headers {
					req.Header.Set(key, value)
				}

				// Send request
				xhr.Set("readyState", 2)
				j.callHandler(xhr, "onreadystatechange")

				client := outboundhttp.NewClient(30 * time.Second)
				resp, err := client.Do(req)
				if err != nil {
					xhr.Set("readyState", 4)
					xhr.Set("status", 0)
					xhr.Set("statusText", err.Error())
					j.callHandler(xhr, "onerror")
					j.callHandler(xhr, "onreadystatechange")
					return
				}
				defer resp.Body.Close()

				// Read response
				xhr.Set("readyState", 3)
				j.callHandler(xhr, "onreadystatechange")

				bodyBytes, err := io.ReadAll(resp.Body)
				if err != nil {
					xhr.Set("readyState", 4)
					xhr.Set("status", resp.StatusCode)
					xhr.Set("statusText", err.Error())
					j.callHandler(xhr, "onerror")
					j.callHandler(xhr, "onreadystatechange")
					return
				}

				// Complete
				xhr.Set("readyState", 4)
				xhr.Set("status", resp.StatusCode)
				xhr.Set("statusText", resp.Status)
				xhr.Set("responseText", string(bodyBytes))
				xhr.Set("response", string(bodyBytes))
				j.callHandler(xhr, "onreadystatechange")
				j.callHandler(xhr, "onload")
			}

			if async {
				go sendFunc()
			} else {
				sendFunc()
			}

			return goja.Undefined()
		})

		// getAllResponseHeaders method
		xhr.Set("getAllResponseHeaders", func(call goja.FunctionCall) goja.Value {
			return j.vm.ToValue("")
		})

		// getResponseHeader method
		xhr.Set("getResponseHeader", func(call goja.FunctionCall) goja.Value {
			return goja.Null()
		})

		return nil
	}
}

// callHandler invokes the event handler
func (j *JavaScriptSender) callHandler(obj *goja.Object, handlerName string) {
	handler := obj.Get(handlerName)
	if handler != nil && !goja.IsUndefined(handler) && !goja.IsNull(handler) {
		if fn, ok := goja.AssertFunction(handler); ok {
			fn(obj)
		}
	}
}
