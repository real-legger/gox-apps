package app

import (
	"github.com/thescaffold/gox-packages/libs/core/image"
)

// AppService is the public assets service. Mirrors ntx-apps/libs/assets/src/app.service.ts.
type AppService struct {
	imageService *image.Service `inject:""`
}

func (s *AppService) GetHello() string {
	return "Hello World!"
}
