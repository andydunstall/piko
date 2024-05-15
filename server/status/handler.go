package status

import "github.com/gin-gonic/gin"

// Handler is a handler in the Piko status API.
//
// The handler registers routes for that expose APIs to inspect the status of
// that component.
type Handler interface {
	// Register registers routes on the given group for the handler.
	Register(group *gin.RouterGroup)
}
