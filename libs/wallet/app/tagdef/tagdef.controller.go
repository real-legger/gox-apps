package tagdef

import (
	"github.com/awesome-goose/goose/types"
	"github.com/thescaffold/gox-packages/libs/core/response"
)

type TagDefController struct {
	svc *TagDefService `inject:""`
}

const title = "tag_definition"

func (c *TagDefController) Create(dto *CreateTagDefReq) types.Output {
	out, err := c.svc.Create(dto.Body)
	if err != nil {
		return response.Conflict(title, err.Error())
	}
	return response.Created(out, title, "tag definition created")
}

func (c *TagDefController) List(dto *ListTagDefReq) types.Output {
	items, err := c.svc.List()
	if err != nil {
		return response.InternalServerError(title, err.Error())
	}
	return response.Success(items, title, "tag definition list", nil)
}

func (c *TagDefController) Get(dto *GetTagDefReq) types.Output {
	out, err := c.svc.Get(dto.Key)
	if err != nil || out == nil {
		return response.NotFound(title, "tag definition not found")
	}
	return response.Success(out, title, "tag definition found", nil)
}

func (c *TagDefController) Update(dto *UpdateTagDefReq) types.Output {
	out, err := c.svc.Update(dto.Key, dto.Body)
	if err != nil {
		return response.InternalServerError(title, err.Error())
	}
	return response.Success(out, title, "tag definition updated", nil)
}

func (c *TagDefController) Delete(dto *DeleteTagDefReq) types.Output {
	if err := c.svc.Delete(dto.Key); err != nil {
		return response.InternalServerError(title, err.Error())
	}
	return response.Success(nil, title, "tag definition deleted", nil)
}
