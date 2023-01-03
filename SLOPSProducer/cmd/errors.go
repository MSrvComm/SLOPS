package main

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (app *Application) logError(err error) {
	app.logger.Println(err)
}

// The errorResponse() method is a generic helper for sending JSON-formatted error
// messages to the client with a given status code. Note that we're using an interface{}
// type for the message parameter, rather than just a string type, as this gives us
// more flexibility over the values that we can include in the response.
func (app *Application) errorResponse(c *gin.Context, status int, msg any) {
	env := envelope{"error": msg}

	err := app.writeJSON(c.Writer, status, env, nil)
	if err != nil {
		app.logError(err)
		c.Writer.WriteHeader(500)
	}
}

// The serverErrorResponse() method will be used when our Application encounters an
// unexpected problem at runtime. It logs the detailed error message, then uses the
// errorResponse() helper to send a 500 Internal Server Error status code and JSON
// response (containing a generic error message) to the client.
func (app *Application) serverErrorResponse(c *gin.Context, err error) {
	app.logError(err)
	message := "the server encountered a problem and could not process your request"
	app.errorResponse(c, http.StatusInternalServerError, message)
}

// The notFoundResponse() method will be used to send a 404 Not Found status code and
// JSON response to the client.
func (app *Application) notFoundResponse(c *gin.Context) {
	message := "the requested resource could not be found"
	app.errorResponse(c, http.StatusNotFound, message)
}

// The methodNotAllowedResponse() method will be used to send a 405 Method Not Allowed
// status code and JSON response to the client.
func (app *Application) methodNotAllowedResponse(c *gin.Context) {
	message := fmt.Sprintf("the %s method is not supported for this resource", c.Request.Method)
	app.errorResponse(c, http.StatusMethodNotAllowed, message)
}

func (app *Application) badRequestResponse(c *gin.Context, err error) {
	app.errorResponse(c, http.StatusBadRequest, err.Error())
}
