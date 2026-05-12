package entry

import (
	"strconv"

	"github.com/awesome-goose/goose/types"
	"github.com/thescaffold/gox-packages/libs/core/response"
)

type EntryController struct {
	svc *EntryService `inject:""`
}

const title = "entry"

func (c *EntryController) List(dto *ListEntriesReq) types.Output {
	limit, _ := strconv.Atoi(dto.Queries["limit"])
	offset, _ := strconv.Atoi(dto.Queries["offset"])
	if limit <= 0 {
		limit = 50
	}
	out, err := c.svc.List(dto.Queries, limit, offset)
	if err != nil {
		return response.InternalServerError(title, err.Error())
	}
	return response.Success(out, title, "entry list", nil)
}

func (c *EntryController) Get(dto *GetEntryReq) types.Output {
	id, perr := strconv.ParseInt(dto.Id, 10, 64)
	if perr != nil {
		return response.BadRequest(title, "invalid entry id")
	}
	out, err := c.svc.Get(id)
	if err != nil || out == nil {
		return response.NotFound(title, "entry not found")
	}
	return response.Success(out, title, "entry found", nil)
}
