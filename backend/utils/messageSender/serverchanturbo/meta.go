package serverchanturbo

import (
	"github.com/komari-monitor/komari/utils/messageSender/factory"
)

// Addition is the configuration item of Server Turbo push channel
// Only the interface address and optional channel/hidden IP/openid are allowed to be configured, fixedly sent in JSON
type Addition struct {
	// APIURL is the complete address of the interface, for example: https://sctapi.ftqq.com/<sendkey>.send
	APIURL string `json:"api_url" required:"true" help:"Full API URL, for example https://sctapi.ftqq.com/<sendkey>.send; see https://sct.ftqq.com/"`
	// Channel is the message channel used for this push, up to two, multiple separated by |, for example: 9|66
	Channel string `json:"channel" help:"Optional message channels separated by |, for example 9|66"`
	// NoIP Whether to hide the calling IP, fill in 1 to hide it
	NoIP string `json:"noip" help:"Hide the caller IP when set to 1; leave empty to show it"`
	// OpenID message carbon copy openid, the test account is separated by ,; the enterprise WeChat application is separated by |
	OpenID string `json:"openid" help:"CC openid; separate test accounts with , and WeCom applications with |"`
}

// Register ServerChan Turbo push channel to the factory
func init() {
	factory.RegisterMessageSender(func() factory.IMessageSender {
		return &ServerChanTurboSender{}
	})
}
