package core

import (
	"net/http"

	"github.com/awesome-goose/goose/types"
	"github.com/thescaffold/gox-packages/libs/core/response"
)

// LedgerError is a structured error code returned by core operations.
// Mirrors the codes in PLAN.md §19.
type LedgerError struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Status  int            `json:"-"`
	Details map[string]any `json:"details,omitempty"`
}

func (e *LedgerError) Error() string {
	if e == nil {
		return ""
	}
	return e.Code + ": " + e.Message
}

// Output renders this error using the standard response envelope.
func (e *LedgerError) Output(title string) types.Output {
	if e == nil {
		return response.InternalServerError(title, "unknown error")
	}
	// Pack details into the message; the envelope's data slot stays nil for errors.
	if len(e.Details) > 0 {
		// Best effort: we only have title/message/code in the envelope.
		// Embed details in message tail for debuggability.
		return response.Error(title, e.Message+" "+jsonish(e.Details), e.Status)
	}
	return response.Error(title, e.Message, e.Status)
}

// Codes
const (
	ErrInvalidRequest             = "INVALID_REQUEST"
	ErrInvalidAmount              = "INVALID_AMOUNT"
	ErrInvalidTag                 = "INVALID_TAG"
	ErrCurrencyNotFound           = "CURRENCY_NOT_FOUND"
	ErrCurrencyMismatch           = "CURRENCY_MISMATCH"
	ErrUnbalancedRow              = "UNBALANCED_ROW"
	ErrInvalidStatusTransition    = "INVALID_STATUS_TRANSITION"
	ErrSameCurrencyConvert        = "SAME_CURRENCY_CONVERT"
	ErrWalletNotFound             = "WALLET_NOT_FOUND"
	ErrAccountNotFound            = "ACCOUNT_NOT_FOUND"
	ErrGroupNotFound              = "GROUP_NOT_FOUND"
	ErrRateNotFound               = "RATE_NOT_FOUND"
	ErrConfigRuleMissing          = "CONFIG_RULE_MISSING"
	ErrAccountFrozen              = "ACCOUNT_FROZEN"
	ErrInsufficientFunds          = "INSUFFICIENT_FUNDS"
	ErrFxIntermediateInsufficient = "FX_INTERMEDIATE_INSUFFICIENT_FUNDS"
	ErrGroupAlreadyReversed       = "GROUP_ALREADY_REVERSED"
	ErrGroupPreconditionFailed    = "GROUP_PRECONDITION_FAILED"
	ErrIdempotencyConflict        = "IDEMPOTENCY_CONFLICT"
	ErrLockTimeout                = "LOCK_TIMEOUT"
	ErrPrimaryEntryMissing        = "PRIMARY_ENTRY_MISSING"
	ErrPrimaryTypeMismatch        = "PRIMARY_TYPE_MISMATCH"
	ErrInternal                   = "INTERNAL_ERROR"
)

func newErr(code, msg string, status int, details map[string]any) *LedgerError {
	return &LedgerError{Code: code, Message: msg, Status: status, Details: details}
}

// Constructors
func ErrBadRequest(code, msg string, details map[string]any) *LedgerError {
	return newErr(code, msg, http.StatusBadRequest, details)
}
func ErrNotFoundf(code, msg string, details map[string]any) *LedgerError {
	return newErr(code, msg, http.StatusNotFound, details)
}
func ErrConflictf(code, msg string, details map[string]any) *LedgerError {
	return newErr(code, msg, http.StatusConflict, details)
}
func ErrInternalf(msg string) *LedgerError {
	return newErr(ErrInternal, msg, http.StatusInternalServerError, nil)
}

// jsonish encodes a small map without depending on encoding/json import bloat.
func jsonish(m map[string]any) string {
	if len(m) == 0 {
		return ""
	}
	out := "{"
	first := true
	for k, v := range m {
		if !first {
			out += ","
		}
		first = false
		out += k + "="
		switch val := v.(type) {
		case string:
			out += val
		default:
			out += sprint(val)
		}
	}
	out += "}"
	return out
}

func sprint(v any) string {
	switch x := v.(type) {
	case int:
		return itoa(int64(x))
	case int64:
		return itoa(x)
	case bool:
		if x {
			return "true"
		}
		return "false"
	case nil:
		return "null"
	default:
		return ""
	}
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
