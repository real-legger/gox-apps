package currency

import (
	"github.com/awesome-goose/goose/types"
	"github.com/thescaffold/gox-packages/libs/core/response"
)

type CurrencyController struct {
	svc *CurrencyService `inject:""`
}

const title = "currency"

func (c *CurrencyController) Create(dto *CreateCurrencyReq) types.Output {
	out, err := c.svc.Create(dto.Body)
	if err != nil {
		return response.Conflict(title, err.Error())
	}
	return response.Created(out, title, "currency created")
}

func (c *CurrencyController) List(dto *ListCurrencyReq) types.Output {
	items, err := c.svc.List()
	if err != nil {
		return response.InternalServerError(title, err.Error())
	}
	return response.Success(items, title, "currency list", nil)
}

func (c *CurrencyController) Get(dto *GetCurrencyReq) types.Output {
	out, err := c.svc.Get(dto.Code)
	if err != nil || out == nil {
		return response.NotFound(title, "currency not found")
	}
	return response.Success(out, title, "currency found", nil)
}

func (c *CurrencyController) Update(dto *UpdateCurrencyReq) types.Output {
	out, err := c.svc.Update(dto.Code, dto.Body)
	if err != nil {
		return response.InternalServerError(title, err.Error())
	}
	return response.Success(out, title, "currency updated", nil)
}

func (c *CurrencyController) Delete(dto *DeleteCurrencyReq) types.Output {
	if err := c.svc.Delete(dto.Code); err != nil {
		return response.InternalServerError(title, err.Error())
	}
	return response.Success(nil, title, "currency deleted", nil)
}
