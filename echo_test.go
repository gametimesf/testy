package testy

import (
	"html/template"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTemplates(t *testing.T) {
	tpl := template.New("testy")
	tpl.Funcs(map[string]any{
		"anchorForResult": anchorForResult,
	})
	_, err := tpl.ParseFS(templateData, "templates/*.gohtml")
	assert.NoError(t, err)
}
