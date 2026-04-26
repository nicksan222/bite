package web

import "github.com/nicksan222/bite/internal/web/routes"

// Type aliases re-exported from internal/web/routes so callers can
// continue to write web.Deps / web.Result / etc. without importing the
// routes package directly. Splitting routes into its own package was an
// internal refactor — the public shape of internal/web is unchanged.
type (
	Deps          = routes.Deps
	Result        = routes.Result
	Table         = routes.Table
	ToolMeta      = routes.ToolMeta
	ParamMeta     = routes.ParamMeta
	NotFoundError = routes.NotFoundError
)
