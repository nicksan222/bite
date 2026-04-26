package web

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"

	"github.com/nicksan222/bite/internal/web/routes"
)

// shutdownTimeout caps how long Listen waits for in-flight requests to
// finish before forcing the listener closed. SSE chat streams can hold
// connections open indefinitely; without a deadline, Ctrl-C on `bite web`
// would hang until the browser disconnects on its own.
const shutdownTimeout = 5 * time.Second

// Server wraps a *fiber.App with bite-specific wiring. New configures the
// router; Listen blocks until ctx is cancelled or the listener errors.
type Server struct {
	app *fiber.App
}

// Config tunes the listener. Zero values are valid: a default of
// 127.0.0.1:8787 keeps the server local-only unless the operator opts in.
type Config struct {
	Host string
	Port int
}

// Addr returns "host:port" with the same defaults Listen would apply.
// Uses net.JoinHostPort so an IPv6 host like "::1" is correctly bracketed.
func (c Config) Addr() string {
	host := c.Host
	if host == "" {
		host = "127.0.0.1"
	}
	port := c.Port
	if port == 0 {
		port = 8787
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

// New builds a *Server with every route registered. It does not start
// the listener — call Listen for that. Returning a configured app makes
// the package testable: tests use app.Test() without binding a port.
func New(deps Deps) *Server {
	app := fiber.New(fiber.Config{
		AppName:      "bite",
		ErrorHandler: errorEnvelope,
	})
	app.Use(requestid.New())
	app.Use(recover.New())
	routes.Register(app, deps)
	return &Server{app: app}
}

// errorEnvelope is fiber's ErrorHandler — converts a returned error into
// the {"error": ...} JSON envelope every other surface in the package
// uses, mapping fiber.Error to its declared code and everything else to
// 500.
func errorEnvelope(c fiber.Ctx, err error) error {
	code := http.StatusInternalServerError
	var fe *fiber.Error
	if errors.As(err, &fe) {
		code = fe.Code
	}
	return c.Status(code).JSON(fiber.Map{"error": err.Error()})
}

// App exposes the underlying *fiber.App for tests, which use app.Test()
// to drive handlers without binding a port. Production callers stick
// to Listen.
func (s *Server) App() *fiber.App { return s.app }

// Listen binds the configured address and serves until ctx is cancelled.
// Cancellation triggers a graceful shutdown; any listener error is
// returned.
func (s *Server) Listen(ctx context.Context, cfg Config) error {
	addr := cfg.Addr()
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.app.Listen(addr, fiber.ListenConfig{DisableStartupMessage: true})
	}()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		_ = s.app.ShutdownWithContext(shutCtx)
		<-errCh
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}
