package main

import (
	"github.com/gin-gonic/gin"
)

func (app *Application) routes() *gin.Engine {
	router := gin.Default()

	router.NoRoute(app.notFoundResponse)
	router.NoMethod(app.methodNotAllowedResponse)
	router.HandleMethodNotAllowed = true

	router.POST("/new", app.NewMessage)

	return router
}
