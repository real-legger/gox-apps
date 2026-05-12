package rate

type RateService struct {
	entity *RateEntity `inject:""`
}

// Latest returns the most recent active rate for a (from, to) pair.
func (s *RateService) Latest(from, to string) (*Rate, error) {
	return s.entity.First(
		"from_currency = ? AND to_currency = ? AND valid_from <= NOW() AND (valid_to IS NULL OR valid_to > NOW())",
		from, to,
	)
}
