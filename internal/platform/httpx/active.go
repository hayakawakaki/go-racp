package httpx

import "github.com/a-h/templ"

var (
	ActiveBase    func(layout Layout, title string) templ.Component
	ActivePage404 func(layout Layout) templ.Component
	ActivePage429 func(layout Layout) templ.Component
)
