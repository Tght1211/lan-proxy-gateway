package aiagent

import (
	"context"

	"github.com/tght/lan-proxy-gateway/internal/app"
	"github.com/tght/lan-proxy-gateway/internal/config"
)

// Controller 是执行器需要的网关能力。*app.App 直接满足它（app 不 import aiagent，无循环）。
type Controller interface {
	Status() app.Status
	HealthText() string
	SetMode(ctx context.Context, mode string) error
	ToggleTUN(ctx context.Context) error
	ToggleAdblock(ctx context.Context) error
	SetGatewayMode(ctx context.Context, mode string) error
	SetSource(ctx context.Context, src config.SourceConfig) error
	AddRule(ctx context.Context, verdict, rule string) error
	Start(ctx context.Context) error
	Stop() error
}
