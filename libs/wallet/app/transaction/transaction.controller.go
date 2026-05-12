package transaction

import (
	"strconv"

	"github.com/awesome-goose/goose/types"
	"github.com/thescaffold/gox-packages/libs/core/response"
)

type TransactionController struct {
	svc *TransactionService `inject:""`
}

const title = "transaction"

func (c *TransactionController) List(dto *ListTransactionsReq) types.Output {
	limit, _ := strconv.Atoi(dto.Queries["limit"])
	offset, _ := strconv.Atoi(dto.Queries["offset"])
	if limit <= 0 {
		limit = 50
	}
	out, err := c.svc.List(dto.Queries, limit, offset)
	if err != nil {
		return response.InternalServerError(title, err.Error())
	}
	return response.Success(out, title, "transaction list", nil)
}

func (c *TransactionController) Get(dto *GetTransactionReq) types.Output {
	id, perr := strconv.ParseInt(dto.Id, 10, 64)
	if perr != nil {
		return response.BadRequest(title, "invalid transaction id")
	}
	out, err := c.svc.Get(id)
	if err != nil || out == nil {
		return response.NotFound(title, "transaction not found")
	}
	return response.Success(out, title, "transaction found", nil)
}

func (c *TransactionController) GetGroup(dto *GetGroupReq) types.Output {
	rows, err := c.svc.ByGroup(dto.GroupId)
	if err != nil {
		return response.InternalServerError(title, err.Error())
	}
	if len(rows) == 0 {
		return response.NotFound(title, "group not found")
	}
	latest := rows[0].Status
	return response.Success(map[string]any{
		"group_id":      dto.GroupId,
		"latest_status": latest,
		"rows":          rows,
	}, title, "group rows", nil)
}
