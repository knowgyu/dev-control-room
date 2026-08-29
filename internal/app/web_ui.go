package app

import (
	_ "embed"
	"net/http"
	"strings"
)

//go:embed ui/index.html
var uiIndexHTML string

//go:embed ui/app.css
var uiAppCSS string

//go:embed ui/app.js
var uiAppJS string

//go:embed ui/PretendardVariable.woff2
var uiPretendardVariable []byte

func writeUI(response http.ResponseWriter, mutationToken string) {
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = strings.NewReader(strings.ReplaceAll(uiIndexHTML, "__MUTATION_TOKEN__", mutationToken)).WriteTo(response)
}

func writeUIAsset(response http.ResponseWriter, contentType, content string) {
	response.Header().Set("Content-Type", contentType+"; charset=utf-8")
	response.Header().Set("Cache-Control", "no-cache")
	_, _ = strings.NewReader(content).WriteTo(response)
}

func writeUIBinaryAsset(response http.ResponseWriter, contentType string, content []byte) {
	response.Header().Set("Content-Type", contentType)
	response.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = response.Write(content)
}
