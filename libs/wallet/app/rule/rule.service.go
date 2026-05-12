package rule

type RuleService struct {
	entity *RuleEntity `inject:""`
}

func (s *RuleService) ByCategory(kind, category string) ([]Rule, error) {
	return s.entity.Some("kind = ? AND category = ? AND active = TRUE", kind, category)
}
