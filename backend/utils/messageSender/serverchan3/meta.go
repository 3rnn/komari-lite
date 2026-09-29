package serverchan3

import (
	"github.com/komari-monitor/komari/utils/messageSender/factory"
)

// Addition is the configuration item of ServerChan3 push channel
// All fields are set as JSON via the admin page
type Addition struct {
	// APIURL is the complete address of the interface, for example: https://<uid>.push.ft07.com/send/<sendkey>.send
	APIURL string `json:"api_url" required:"true" help:"Full API URL, for example https://<uid>.push.ft07.com/send/<sendkey>.send; see https://sc3.ft07.com/"`
	// Tags are optional tags, separated by |, for example: tag1|tag2|tag3
	Tags string `json:"tags" help:"Optional tags separated by |, for example tag1|tag2|tag3"`
}

// Register ServerChan3 Push channel to factory
func init() {
	factory.RegisterMessageSender(func() factory.IMessageSender {
		return &ServerChan3Sender{}
	})
}
