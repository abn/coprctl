package sitewiki

import (
	"embed"
)

//go:embed wiki.css search.js
var cssFS embed.FS

// CSS returns the wiki stylesheet.
func CSS() ([]byte, error) {
	return cssFS.ReadFile("wiki.css")
}

// SearchJS returns the client search script.
func SearchJS() ([]byte, error) {
	return cssFS.ReadFile("search.js")
}
