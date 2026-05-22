package state

import (
	"html/template"

	"github.com/hayakawakaki/go-racp/internal/features/news/app"
	"github.com/hayakawakaki/go-racp/internal/features/news/domain"
)

type NewsListState struct {
	SelectedCategory string
	Items            []app.NewsItem
	Categories       []domain.Category
	CanManage        bool
}

type NewsDetailState struct {
	BodyHTML  template.HTML
	Item      app.NewsItem
	CanManage bool
}

type NewsFormState struct {
	Errors         map[string]string
	Action         string
	PageTitle      string
	Submit         string
	Title          string
	Body           string
	Category       string
	InitialPreview template.HTML
	Categories     []domain.Category
}
