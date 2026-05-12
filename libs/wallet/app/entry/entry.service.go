package entry

type EntryService struct {
	entity *EntryEntity `inject:""`
}

func (s *EntryService) Get(id any) (*Entry, error) {
	return s.entity.First("id = ?", id)
}

func (s *EntryService) ByTransaction(txId any) ([]Entry, error) {
	return s.entity.Some("transaction_id = ?", txId)
}

func (s *EntryService) ByAccount(accountId any, limit, offset int) ([]Entry, error) {
	return s.entity.Find(offset, limit, "account_id = ?", accountId)
}

func (s *EntryService) List(filter map[string]string, limit, offset int) ([]Entry, error) {
	if v, ok := filter["transaction_id"]; ok && v != "" {
		return s.entity.Find(offset, limit, "transaction_id = ?", v)
	}
	if v, ok := filter["account_id"]; ok && v != "" {
		return s.entity.Find(offset, limit, "account_id = ?", v)
	}
	return s.entity.Find(offset, limit, "")
}
