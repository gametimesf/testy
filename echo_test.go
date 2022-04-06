package testy

import (
	"html/template"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTemplates(t *testing.T) {
	_, err := template.ParseFS(templateData, "templates/*.gohtml")
	assert.NoError(t, err)
}
