package server

import (
	"testing"
)

func TestBasicInfoCarriesOnlyFieldsThePanelKnows(t *testing.T) {
	data := buildBasicInfoMap()

	if data["remote_protocol"] != 2 {
		t.Fatalf("remote_protocol = %#v, want 2", data["remote_protocol"])
	}

	// 面板按 Agent 上报的键直接更新 clients 表：多一个不存在的列就会拼出
	// "no such column"，整条基础信息被丢弃（实测：remote_control_enabled 会让面板
	// 永远看不到这台机器的 CPU/内存/系统信息）。精简版已无远程控制，一律不得上报。
	for _, forbidden := range []string{
		"remote_control_enabled",
		"remote_control_protected",
		"mcp_full",
		"mcp_full_version",
		"mcp_enabled",
	} {
		if _, ok := data[forbidden]; ok {
			t.Fatalf("basic info must not include %s; the panel has no such column and would reject the whole update", forbidden)
		}
	}
}
