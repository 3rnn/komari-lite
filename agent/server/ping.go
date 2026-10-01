package server

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	v2 "github.com/nuomiiiii/lite-agent/protocol/v2"
	"github.com/nuomiiiii/lite-agent/ws"
	probing "github.com/prometheus-community/pro-bing"
)

const pingTimeout = 3 * time.Second

func handlePingEvent(conn *ws.SafeConn, raw any) bool {
	var params struct {
		TaskID uint   `json:"ping_task_id"`
		Type   string `json:"ping_type"`
		Target string `json:"ping_target"`
	}
	if err := v2.BindParams(raw, &params); err != nil || params.TaskID == 0 || strings.TrimSpace(params.Target) == "" || (params.Type != "tcp" && params.Type != "http" && params.Type != "icmp") {
		log.Println("ignored invalid ping event")
		return false
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
		defer cancel()
		value := probeLatency(ctx, params.Type, params.Target)
		result := v2.BuildPingResultPayload(params.TaskID, params.Type, value, time.Now().UTC())
		if conn != nil {
			if err := conn.WriteJSON(result); err == nil {
				return
			} else {
				log.Printf("failed to send ping result for task %d over WebSocket, trying HTTP: %v", params.TaskID, err)
			}
		}
		payload, err := json.Marshal(result)
		if err != nil {
			log.Printf("failed to encode ping result for task %d: %v", params.TaskID, err)
			return
		}
		reportCtx, reportCancel := context.WithTimeout(context.Background(), pingTimeout)
		defer reportCancel()
		if _, err := postV2RequestContext(reportCtx, payload); err != nil {
			log.Printf("failed to POST ping result for task %d: %v", params.TaskID, err)
		}
	}()
	return true
}

func probeLatency(ctx context.Context, kind, target string) int {
	start := time.Now()
	switch kind {
	case "tcp":
		connection, err := (&net.Dialer{Timeout: pingTimeout}).DialContext(ctx, "tcp", target)
		if err != nil {
			return -1
		}
		connection.Close()
	case "http":
		parsed, err := url.Parse(target)
		if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return -1
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return -1
		}
		client := &http.Client{Timeout: pingTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		response, err := client.Do(request)
		if err != nil {
			return -1
		}
		response.Body.Close()
	case "icmp":
		pinger := probing.New(target)
		pinger.Count = 1
		pinger.Timeout = pingTimeout
		pinger.ResolveTimeout = pingTimeout
		pinger.SetPrivileged(os.Geteuid() == 0)
		if err := pinger.RunWithContext(ctx); err != nil || pinger.Statistics().PacketsRecv == 0 {
			return -1
		}
		return positiveMilliseconds(pinger.Statistics().AvgRtt)
	default:
		return -1
	}
	return positiveMilliseconds(time.Since(start))
}

func positiveMilliseconds(elapsed time.Duration) int {
	if elapsed < time.Millisecond {
		return 1
	}
	return int(elapsed.Milliseconds())
}
