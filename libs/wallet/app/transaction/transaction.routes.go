package transaction

import "github.com/awesome-goose/goose/modules/router"

var ROUTES = router.ForRoutes(
	router.Get("/transactions", []any{TransactionController{}, "List"}),
	router.Get("/transactions/:id", []any{TransactionController{}, "Get"}),
	router.Get("/groups/:group_id", []any{TransactionController{}, "GetGroup"}),
)
