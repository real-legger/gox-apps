package transaction

type TransactionService struct {
	entity *TransactionEntity `inject:""`
}

func (s *TransactionService) Get(id any) (*Transaction, error) {
	return s.entity.First("id = ?", id)
}

func (s *TransactionService) List(filter map[string]string, limit, offset int) ([]Transaction, error) {
	where, args := buildWhere(filter)
	return s.entity.Find(offset, limit, where, args...)
}

func (s *TransactionService) ByGroup(groupId string) ([]Transaction, error) {
	return s.entity.Some("group_id = ?", groupId)
}

func buildWhere(filter map[string]string) (string, []any) {
	var clauses []string
	var args []any
	keys := []string{"group_id", "status", "type", "currency", "primary_account_id"}
	for _, k := range keys {
		if v, ok := filter[k]; ok && v != "" {
			clauses = append(clauses, k+" = ?")
			args = append(args, v)
		}
	}
	if len(clauses) == 0 {
		return "", nil
	}
	out := clauses[0]
	for i := 1; i < len(clauses); i++ {
		out += " AND " + clauses[i]
	}
	return out, args
}
