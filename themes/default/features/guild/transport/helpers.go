package transport

import (
	"fmt"
	"net/url"
)

func pageURL(baseURL string, page int, query string) string {
	return fmt.Sprintf("%s?page=%d&q=%s", baseURL, page, url.QueryEscape(query))
}
