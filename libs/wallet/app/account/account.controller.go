package account

import (
	"strconv"

	"github.com/awesome-goose/goose/types"
	"github.com/thescaffold/gox-packages/libs/core/response"
)

type AccountController struct {
	svc *AccountService `inject:""`
}

const title = "account"

func (c *AccountController) Create(dto *CreateAccountsDto) types.Output {
	out, err := c.svc.Create(dto.Accounts)
	if err != nil {
		return response.Conflict(title, err.Error())
	}
	return response.Created(map[string]any{"accounts": out}, title, "accounts created")
}

func (c *AccountController) List(dto *ListAccountsReq) types.Output {
	walletId := dto.Queries["wallet_id"]
	out, err := c.svc.List(walletId)
	if err != nil {
		return response.InternalServerError(title, err.Error())
	}
	return response.Success(out, title, "account list", nil)
}

func (c *AccountController) Get(dto *GetAccountReq) types.Output {
	id, perr := strconv.ParseInt(dto.Id, 10, 64)
	if perr != nil {
		return response.BadRequest(title, "invalid account id")
	}
	out, err := c.svc.Get(id)
	if err != nil || out == nil {
		return response.NotFound(title, "account not found")
	}
	return response.Success(out, title, "account found", nil)
}

func (c *AccountController) UpdateStatus(dto *UpdateStatusReq) types.Output {
	id, perr := strconv.ParseInt(dto.Id, 10, 64)
	if perr != nil {
		return response.BadRequest(title, "invalid account id")
	}
	if dto.Body.Status != "active" && dto.Body.Status != "frozen" && dto.Body.Status != "closed" {
		return response.BadRequest(title, "status must be active|frozen|closed")
	}
	if err := c.svc.SetStatus(id, dto.Body.Status); err != nil {
		return response.InternalServerError(title, err.Error())
	}
	return response.Success(map[string]any{"id": id, "status": dto.Body.Status}, title, "status updated", nil)
}
