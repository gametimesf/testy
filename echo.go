package testy

import (
	"embed"
	"html/template"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
)

//go:embed templates/*
var templateData embed.FS

type listResultsCtx struct {
	Results   []Summary
	PrevPages []int
	Page      int
	NextPage  int
	More      bool
}

type echoRenderer struct {
	templates *template.Template
}

// Render implements echo.Renderer and renders the request template to the response writer.
func (er echoRenderer) Render(w io.Writer, name string, data any, _ echo.Context) error {
	return er.templates.ExecuteTemplate(w, name, data)
}

// EchoRenderer loads the HTML templates and returns an echo.Renderer for the routes provided by this package. Assign
// this to the Renderer field of your Echo app (or wrap it with your own)
func EchoRenderer() (echo.Renderer, error) {
	t, err := template.ParseFS(templateData, "templates/*.gohtml")
	if err != nil {
		return nil, err
	}
	return echoRenderer{templates: t}, nil
}

// AddEchoRoutes adds routes to an Echo router that can run tests and retrieve tests results.
func AddEchoRoutes(router *echo.Group) {
	router.GET("/run", runTests)
	results := router.Group("/results")
	results.GET("", listResults)
	results.GET("/", listResults)
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

func listResults(c echo.Context) error {
	if instance.db == nil {
		return c.String(http.StatusInternalServerError, "No test result database configured.")
	}

	req := struct {
		Page int `query:"page"`
	}{}
	err := c.Bind(&req)
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	if req.Page < 0 {
		return c.String(http.StatusBadRequest, "Page must be positive")
	}
	if req.Page == 0 {
		req.Page = 1
	}

	results, more, err := instance.db.Enumerate(c.Request().Context(), req.Page)
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	prevPages := make([]int, 0, req.Page-1)
	for i := 1; i < req.Page; i++ {
		prevPages = append(prevPages, i)
	}

	return c.Render(http.StatusOK, "result_list.gohtml", listResultsCtx{
		Results:   results,
		More:      more,
		PrevPages: prevPages,
		Page:      req.Page,
		NextPage:  req.Page + 1,
	})
}
