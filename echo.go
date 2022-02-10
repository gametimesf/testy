package testy

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// AddEchoRoutes adds routes to an Echo router that can run tests and retrieve tests results.
func AddEchoRoutes(router *echo.Group) {
	router.GET("/run", runTests)
}

func runTests(c echo.Context) error {
	results := Run()
	// TODO convert results into a better format?
	return c.JSON(http.StatusOK, results)
}
