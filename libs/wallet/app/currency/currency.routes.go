package currency

import "github.com/awesome-goose/goose/modules/router"

var ROUTES = router.ForRoutes(
	router.Post("/currencies", []any{CurrencyController{}, "Create"}),
	router.Get("/currencies", []any{CurrencyController{}, "List"}),
	router.Get("/currencies/:code", []any{CurrencyController{}, "Get"}),
	router.Patch("/currencies/:code", []any{CurrencyController{}, "Update"}),
	router.Delete("/currencies/:code", []any{CurrencyController{}, "Delete"}),
)
