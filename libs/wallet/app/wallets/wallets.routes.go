package wallets

import "github.com/awesome-goose/goose/modules/router"

var ROUTES = router.ForRoutes(
	router.Resource("wallets", WalletController{}).All(),
)
