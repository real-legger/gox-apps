package core

import (
	"encoding/json"

	"gorm.io/gorm"

	"github.com/awesome-goose/goose/modules/sql"
)

// Service is the wallet app's top-level service. It re-exports the
// core service for convenience.
type Service struct {
	db     *sql.Db `inject:""`
	layer0 *Layer0 `inject:""`
}

// Service is the core application service. It owns higher-level
// orchestration that does not belong to either Layer0 or Layer1 — for now,
// account creation with vocabulary validation and a thin transactional
// wrapper around append_transaction_rows for HTTP callers.

// CreateAccounts inserts new accounts and returns the created rows.
//
// Implements PLAN.md §17.1 create_accounts:
//   - Validates account class.
//   - Validates tags against tag_definitions vocabulary.
func (s *Service) CreateAccounts(inputs []CreateAccountInput) ([]Account, *LedgerError) {
	if len(inputs) == 0 {
		return nil, ErrBadRequest(ErrInvalidRequest, "accounts is empty", nil)
	}
	out := make([]Account, 0, len(inputs))
	for i, in := range inputs {
		if err := ValidateAccountClass(in.Class); err != nil {
			err.Details = MergeDetails(err.Details, map[string]any{"index": i})
			return nil, err
		}
		if lerr := ValidateTagsAgainstVocabulary(s.db.DB, in.Tags, "account"); lerr != nil {
			lerr.Details = MergeDetails(lerr.Details, map[string]any{"index": i})
			return nil, lerr
		}

		metaRaw, jerr := json.Marshal(in.Metadata)
		if jerr != nil || string(metaRaw) == "null" {
			metaRaw = []byte("{}")
		}

		var r AccountRow
		err := s.db.DB.Raw(
			`INSERT INTO "WalletAccounts" (wallet_id, currency, class, tags, status, balance, available_balance, metadata)
			 VALUES (?, ?, ?, ?::jsonb, 'active', 0, 0, ?::jsonb)
			 RETURNING id, wallet_id, currency, class, status, balance, available_balance, tags, metadata, created_at`,
			in.WalletID, in.Currency, in.Class, JsonOrEmpty(in.Tags), string(metaRaw),
		).Scan(&r).Error
		if err != nil {
			return nil, ErrInternalf("create account failed: " + err.Error())
		}
		out = append(out, r.ToAccount())
	}
	return out, nil
}

// AppendRows wraps Layer0.AppendTransactionRows in a transaction so HTTP
// callers can hit Layer 0 directly without managing the tx themselves.
func (s *Service) AppendRows(rows []RowInput) ([]AppendResult, *LedgerError) {
	var results []AppendResult
	var lerr *LedgerError
	txErr := s.db.DB.Transaction(func(tx *gorm.DB) error {
		r, e := s.layer0.AppendTransactionRows(tx, AppendOpts{Rows: rows})
		if e != nil {
			lerr = e
			return e
		}
		results = r
		return nil
	})
	if txErr != nil {
		if lerr != nil {
			return nil, lerr
		}
		return nil, ErrInternalf("transaction failed: " + txErr.Error())
	}
	return results, nil
}
