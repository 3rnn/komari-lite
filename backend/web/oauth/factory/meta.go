package factory

import "github.com/gin-gonic/gin"

type IOidcProvider interface {
	GetName() string
	// Return a *Configuration (for example, &Configuration{}).
	GetConfiguration() Configuration
	// Return the authorization URL and state.
	GetAuthorizationURL(redirectURI string) (string, string)
	OnCallback(ctx *gin.Context, state string, query map[string]string, callbackURI string) (OidcCallback, error)
	Init() error
	Destroy() error
}

type OidcCallback struct {
	UserId string
}

type Configuration interface{}

type OidcConstructor func() IOidcProvider
