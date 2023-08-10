package main

import (
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/gametimesf/testy"
	_ "github.com/gametimesf/testy/example/tests" // register the test cases
)

const port = 12345

func main() {
	r, err := testy.EchoRenderer()
	if err != nil {
		panic(err)
	}

	api := echo.New()
	api.Renderer = r
	api.Use(middleware.Logger())

	tests := api.Group("/tests")
	testy.SetDB(&testy.InMemoryDB{})
	testy.AddEchoRoutes(tests)

	err = api.Start(fmt.Sprintf(":%d", port))
	if err != nil {
		panic(err)
	}
}
