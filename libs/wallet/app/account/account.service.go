package account

import (
	"github.com/real-legger/gox-apps/libs/wallet/pkg/core"
)

type AccountService struct {
	entity *AccountEntity `inject:""`
	core   *core.Service  `inject:""`
}

// Create proxies to core.Service.CreateAccounts so tag-vocabulary
// validation lives in one place.
func (s *AccountService) Create(inputs []CreateAccountInput) ([]Account, error) {
	mapped := make([]core.CreateAccountInput, len(inputs))
	for i, in := range inputs {
		mapped[i] = core.CreateAccountInput{
			WalletID: in.WalletId,
			Currency: in.Currency,
			Class:    in.Class,
			Tags:     core.Tags(in.Tags),
			Metadata: in.Metadata,
		}
	}
	created, lerr := s.core.CreateAccounts(mapped)
	if lerr != nil {
		return nil, lerr
	}
	out := make([]Account, len(created))
	for i, a := range created {
		out[i] = fromLedger(a)
	}
	return out, nil
}

func (s *AccountService) Get(id any) (*Account, error) {
	return s.entity.First("id = ?", id)
}

func (s *AccountService) List(walletId string) ([]Account, error) {
	if walletId == "" {
		return s.entity.All()
	}
	return s.entity.Some("wallet_id = ?", walletId)
}

// SetStatus updates only the status column (active/frozen/closed).
func (s *AccountService) SetStatus(id any, status string) error {
	patch := &Account{Status: status}
	_, err := s.entity.Update(patch, "id = ?", id)
	return err
}

func fromLedger(a core.Account) Account {
	out := Account{
		Id:               a.ID,
		WalletId:         a.WalletID,
		Currency:         a.Currency,
		Class:            string(a.Class),
		Status:           string(a.Status),
		Balance:          a.Balance,
		AvailableBalance: a.AvailableBalance,
		Metadata:         a.Metadata,
	}
	if !a.CreatedAt.IsZero() {
		ct := a.CreatedAt
		out.CreatedAt = &ct
	}
	if len(a.Tags) > 0 {
		// Re-encode the typed Tags map back into JSONB-friendly bytes.
		// json.Marshal here cannot fail.
		b, _ := jsonMarshal(a.Tags)
		out.Tags = b
	}
	return out
}
