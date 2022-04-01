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

	if instance.db != nil {
		// TODO do we want to alert this somehow?
		// doesn't make sense to return an http error since we do have test results
		_, _ = SaveResult(c.Request().Context(), results)
	}

	// TODO convert results into a better format?
	return c.JSON(http.StatusOK, results)
}
