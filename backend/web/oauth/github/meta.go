package github

import (
	"github.com/komari-monitor/komari/web/oauth/factory"
	"github.com/patrickmn/go-cache"
)

func init() {
	factory.RegisterOidcProvider(func() factory.IOidcProvider {
		return &Github{}
	})
}

type Github struct {
	Addition
	stateCache *cache.Cache // Mappings for storing state and user information
}

type Addition struct {
	ClientId     string `json:"client_id" required:"true"`
	ClientSecret string `json:"client_secret" required:"true"`
}

type GitHubUser struct {
	ID    int    `json:"id"`
	Login string `json:"login"`
	Email string `json:"email"`
}
