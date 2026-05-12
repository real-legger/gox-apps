package rate

import "github.com/awesome-goose/goose/modules/router"

var ROUTES = router.ForRoutes(
	router.Resource("rates", RateController{}).All(),
)
