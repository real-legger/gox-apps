package app

import "github.com/awesome-goose/goose/modules/router"

var ROUTES = router.ForRoutes(
	router.Get("/", []any{AppController{}, "Health"}),

	// router.Post("/transactions/append", []any{AppController{}, "AppendRows"}),
	router.Post("/lien", []any{AppController{}, "Lien"}),
	router.Post("/execute", []any{AppController{}, "Execute"}),
	router.Post("/execute_direct", []any{AppController{}, "ExecuteDirect"}),
	router.Post("/reverse", []any{AppController{}, "Reverse"}),
	router.Post("/convert", []any{AppController{}, "Convert"}),
)
