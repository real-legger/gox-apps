package account

import "github.com/awesome-goose/goose/modules/router"

var ROUTES = router.ForRoutes(
	router.Post("/accounts", []any{AccountController{}, "Create"}),
	router.Get("/accounts", []any{AccountController{}, "List"}),
	router.Get("/accounts/:id", []any{AccountController{}, "Get"}),
	router.Patch("/accounts/:id/status", []any{AccountController{}, "UpdateStatus"}),
)
