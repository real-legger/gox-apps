package tagdef

type TagDefService struct {
	entity *TagDefEntity `inject:""`
}

func (s *TagDefService) Create(in CreateTagDefinitionDto) (*TagDefinition, error) {
	t := &TagDefinition{
		Key:           in.Key,
		Description:   in.Description,
		AppliesTo:     in.AppliesTo,
		AllowedValues: in.AllowedValues,
	}
	if err := s.entity.Insert(t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *TagDefService) Get(key string) (*TagDefinition, error) {
	return s.entity.First("key = ?", key)
}

func (s *TagDefService) List() ([]TagDefinition, error) {
	return s.entity.All()
}

func (s *TagDefService) Update(key string, in UpdateTagDefinitionDto) (*TagDefinition, error) {
	patch := &TagDefinition{}
	if in.Description != nil {
		patch.Description = in.Description
	}
	if in.AppliesTo != nil {
		patch.AppliesTo = *in.AppliesTo
	}
	if in.AllowedValues != nil {
		patch.AllowedValues = in.AllowedValues
	}
	if _, err := s.entity.Update(patch, "key = ?", key); err != nil {
		return nil, err
	}
	return s.Get(key)
}

func (s *TagDefService) Delete(key string) error {
	_, err := s.entity.Delete("key = ?", key)
	return err
}
