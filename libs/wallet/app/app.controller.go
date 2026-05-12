package app

import (
	"encoding/json"

	"github.com/awesome-goose/goose/types"
	"github.com/real-legger/gox-apps/libs/wallet/pkg/core"
	"github.com/thescaffold/gox-packages/libs/core/response"
)

type AppController struct {
	appService *AppService   `inject:""`
	core       *core.Service `inject:""`

	log    types.Log    `inject:""`
	layer1 *core.Layer1 `inject:""`
}

func (c *AppController) Health(dto *HealthDto) types.Output {
	return response.Success(map[string]any{"status": c.appService.GetHello()}, "wallet", "ok", nil)
}

const titleWallet = "wallet"

// ─── Layer 0 ─────────────────────────────────────────────────────────────────

func (c *AppController) AppendRows(dto *core.AppendRowsDto) types.Output {
	results, err := c.core.AppendRows(dto.Rows)
	if err != nil {
		return err.Output(titleWallet)
	}
	return response.Created(map[string]any{"rows": results}, titleWallet, "rows appended")
}

// ─── Layer 1 ─────────────────────────────────────────────────────────────────

func (c *AppController) Lien(dto *core.LienDto) types.Output {
	raw, err := c.layer1.Lien(dto.Body)
	if err != nil {
		return err.Output(titleWallet)
	}
	return rawSuccess(raw, "lien opened")
}

func (c *AppController) Execute(dto *core.ExecuteDto) types.Output {
	raw, err := c.layer1.Execute(dto.Body)
	if err != nil {
		return err.Output(titleWallet)
	}
	return rawSuccess(raw, "lien executed")
}

func (c *AppController) ExecuteDirect(dto *core.ExecuteDirectDto) types.Output {
	raw, err := c.layer1.ExecuteDirect(dto.Body)
	if err != nil {
		return err.Output(titleWallet)
	}
	return rawSuccess(raw, "executed direct")
}

func (c *AppController) Reverse(dto *core.ReverseDto) types.Output {
	raw, err := c.layer1.Reverse(dto.Body)
	if err != nil {
		return err.Output(titleWallet)
	}
	return rawSuccess(raw, "reversed")
}

func (c *AppController) Convert(dto *core.ConvertDto) types.Output {
	raw, err := c.layer1.Convert(dto.Body)
	if err != nil {
		return err.Output(titleWallet)
	}
	return rawSuccess(raw, "converted")
}

// rawSuccess wraps a JSON-serialised result so we don't need to round-trip
// it through map[string]any just to put it back in the envelope.
func rawSuccess(raw json.RawMessage, msg string) types.Output {
	var data any
	if err := json.Unmarshal(raw, &data); err != nil {
		return response.InternalServerError(titleWallet, "marshal response failed")
	}
	return response.Success(data, titleWallet, msg, nil)
}
