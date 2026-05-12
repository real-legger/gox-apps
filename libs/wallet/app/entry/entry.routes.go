package entry

import "github.com/awesome-goose/goose/modules/router"

var ROUTES = router.ForRoutes(
	router.Get("/entries", []any{EntryController{}, "List"}),
	router.Get("/entries/:id", []any{EntryController{}, "Get"}),
)
