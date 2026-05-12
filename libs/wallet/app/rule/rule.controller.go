package rule

import (
	"github.com/thescaffold/gox-packages/libs/core/crud"
)

type RuleController struct {
	crud.CrudResource[Rule, CreateRuleDto, UpdateRuleDto]

	entity *RuleEntity `inject:""`
}

func (c *RuleController) OnRegister() {
	c.Hydrate(c.entity, crud.Config[Rule, CreateRuleDto, UpdateRuleDto]{
		Name:       "Rule",
		Searchable: []string{"category", "kind"},
	})
}
