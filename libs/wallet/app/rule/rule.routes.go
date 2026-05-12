package rule

import "github.com/awesome-goose/goose/modules/router"

var ROUTES = router.ForRoutes(
	router.Resource("rules", RuleController{}).All(),
)
