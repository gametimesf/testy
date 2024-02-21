package testy

import (
	"embed"
	"errors"
	"html/template"
	"io"
	"net/http"
	"regexp"
	"time"

	"github.com/labstack/echo/v4"
)

//go:embed templates/*
var templateData embed.FS

type listResultsCtx struct {
	echo      *echo.Echo
	Results   []Summary
	PrevPages []int
	Page      int
	NextPage  int
	More      bool
}

type showResultCtx struct {
	Result TestResult
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
	tpl := template.New("testy")
	tpl.Funcs(map[string]any{
		"anchorForResult": anchorForResult,
	})
	tpl, err := tpl.ParseFS(templateData, "templates/*.gohtml")
	if err != nil {
		return nil, err
	}
	return echoRenderer{templates: tpl}, nil
}

// AddEchoRoutes adds routes to an Echo router that can run tests and retrieve tests results.
func AddEchoRoutes(router *echo.Group) {
	router.GET("/run", runTests)

	results := router.Group("/results")
	results.GET("", listResults)
	results.GET("/", listResults)
	results.GET("/:id", showResult).Name = "showResult"
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
		echo:      c.Echo(),
		Results:   results,
		More:      more,
		PrevPages: prevPages,
		Page:      req.Page,
		NextPage:  req.Page + 1,
	})
}

func showResult(c echo.Context) error {
	if instance.db == nil {
		return c.String(http.StatusInternalServerError, "No test result database configured.")
	}

	req := struct {
		ID  string `param:"id"`
		Raw bool   `query:"raw"`
	}{}
	err := c.Bind(&req)
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}

	tr, err := LoadResult(c.Request().Context(), req.ID)
	if errors.Is(err, ErrNotFound) {
		return c.NoContent(http.StatusNotFound)
	}
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	if req.Raw {
		return c.JSON(http.StatusOK, tr)
	}

	tr.Started = tr.Started.Truncate(time.Second)
	return c.Render(http.StatusOK, "result.gohtml", showResultCtx{
		Result: tr,
	})
}

func (c listResultsCtx) LinkForID(id string) string {
	return c.echo.Reverse("showResult", id)
}

var anchorRegex = regexp.MustCompile(`[^a-zA-Z0-9._:/'()-]`)

func anchorForResult(pkg, name string) string {
	return string(anchorRegex.ReplaceAll([]byte(pkg+"/"+name), []byte{'_'}))
}
