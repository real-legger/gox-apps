package currency

type CurrencyService struct {
	entity *CurrencyEntity `inject:""`
}

func (s *CurrencyService) Create(in CreateCurrencyDto) (*Currency, error) {
	c := &Currency{Code: in.Code, Exponent: in.Exponent, Name: in.Name}
	if err := s.entity.Insert(c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *CurrencyService) Get(code string) (*Currency, error) {
	return s.entity.First("code = ?", code)
}

func (s *CurrencyService) List() ([]Currency, error) {
	return s.entity.All()
}

func (s *CurrencyService) Update(code string, in UpdateCurrencyDto) (*Currency, error) {
	patch := &Currency{}
	if in.Exponent != nil {
		patch.Exponent = *in.Exponent
	}
	if in.Name != nil {
		patch.Name = *in.Name
	}
	if _, err := s.entity.Update(patch, "code = ?", code); err != nil {
		return nil, err
	}
	return s.Get(code)
}

func (s *CurrencyService) Delete(code string) error {
	_, err := s.entity.Delete("code = ?", code)
	return err
}
