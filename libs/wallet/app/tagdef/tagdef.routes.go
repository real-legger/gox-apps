package tagdef

import "github.com/awesome-goose/goose/modules/router"

var ROUTES = router.ForRoutes(
	router.Post("/tag-definitions", []any{TagDefController{}, "Create"}),
	router.Get("/tag-definitions", []any{TagDefController{}, "List"}),
	router.Get("/tag-definitions/:key", []any{TagDefController{}, "Get"}),
	router.Patch("/tag-definitions/:key", []any{TagDefController{}, "Update"}),
	router.Delete("/tag-definitions/:key", []any{TagDefController{}, "Delete"}),
)
