package qq

import (
	"github.com/komari-monitor/komari/web/oauth/factory"
	"github.com/patrickmn/go-cache"
)

func init() {
	factory.RegisterOidcProvider(func() factory.IOidcProvider {
		return &QQ{}
	})
}

type QQ struct {
	Addition
	stateCache *cache.Cache // Mappings for storing state and user information
}

type Addition struct {
	AggregationURL string `json:"aggregation_url" required:"true" default:"https://login.qjqq.cn"` // Aggregated login URL
	AppId          string `json:"app_id" required:"true"`
	AppKey         string `json:"app_key" required:"true"`
	LoginType      string `json:"login_type" required:"true"` // Login method, such as qq, google, etc.
}
