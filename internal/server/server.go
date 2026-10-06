package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	"github.com/Pleasurecruise/eyeful/internal/apperror"
	"github.com/Pleasurecruise/eyeful/internal/auth"
)

type Handler interface {
	Register(g *echo.Group)
}

type Options struct {
	Authenticate   func(ctx context.Context, bearer, session string) (auth.Principal, error)
	PublicRoutes   []string
	Assets         http.FileSystem
	RateLimit      int
	TrustedOrigins []string
	CORSOrigins    []string
}

const consoleRoute = "/*"

type Server struct {
	echo *echo.Echo
	log  *slog.Logger
}

func New(log *slog.Logger, opts Options, handlers ...Handler) *Server {
	e := echo.New()
	e.Logger = log
	e.HTTPErrorHandler = errorHandler(log)
	e.Use(middleware.RequestID())
	e.Use(middleware.Recover())
	e.Use(accessLog(log))
	if len(opts.CORSOrigins) > 0 {
		e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins:     opts.CORSOrigins,
			AllowCredentials: true,
			AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete},
			AllowHeaders:     []string{echo.HeaderContentType, echo.HeaderAuthorization, "Idempotency-Key"},
			ExposeHeaders:    []string{echo.HeaderXRequestID, echo.HeaderRetryAfter, echo.HeaderLocation},
			MaxAge:           600,
		}))
	}
	e.Use(middleware.BodyLimit(1 << 20))
	e.Use(RateLimit("api", opts.RateLimit, time.Minute))
	e.Use(authenticate(opts.Authenticate, opts.PublicRoutes, opts.TrustedOrigins))
	// TODO(observability): OpenTelemetry tracing, one span per request and per review stage.

	g := e.Group("")
	for _, h := range handlers {
		h.Register(g)
	}
	if opts.Assets != nil {
		e.GET(consoleRoute, spa(opts.Assets))
	}
	return &Server{echo: e, log: log.With(slog.String("component", "server"))}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.echo.ServeHTTP(w, r) }

func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	sc := echo.StartConfig{
		Listener:        ln,
		HideBanner:      true,
		HidePort:        true,
		GracefulTimeout: 10 * time.Second,
		ListenerAddrFunc: func(a net.Addr) {
			s.log.Info("listening", slog.String("addr", a.String()))
		},
	}
	err := sc.Start(ctx, s.echo)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("serve http: %w", err)
	}
	return nil
}

// TODO(auth): GitHub App installation tokens for webhook-triggered reviews.
func authenticate(fn func(ctx context.Context, bearer, session string) (auth.Principal, error), public, trusted []string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if c.Path() == consoleRoute {
				return next(c)
			}
			route := c.Request().Method + " " + c.Path()
			if slices.Contains(public, route) {
				return next(c)
			}
			r := c.Request()
			bearer, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok {
				bearer = ""
			}
			session := ""
			if cookie, err := r.Cookie(auth.SessionCookie); err == nil {
				session = cookie.Value
			}
			p, err := fn(r.Context(), bearer, session)
			if errors.Is(err, auth.ErrUnauthenticated) {
				return apperror.New(apperror.CodeUnauthenticated, http.StatusUnauthorized, "sign in or send an API key", nil)
			}
			if err != nil {
				return err
			}
			method := c.Request().Method
			unsafe := method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions
			if p.SessionID != "" && unsafe && !slices.Contains(trusted, c.Request().Header.Get(echo.HeaderOrigin)) {
				return apperror.New(apperror.CodeOriginForbidden, http.StatusForbidden, "changes with a session cookie must come from a trusted origin", nil)
			}
			c.SetRequest(c.Request().WithContext(auth.WithPrincipal(c.Request().Context(), p)))
			return next(c)
		}
	}
}

func accessLog(log *slog.Logger) echo.MiddlewareFunc {
	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogMethod:    true,
		LogURIPath:   true,
		LogStatus:    true,
		LogLatency:   true,
		LogRequestID: true,
		HandleError:  true,
		LogValuesFunc: func(c *echo.Context, v middleware.RequestLoggerValues) error {
			attrs := []slog.Attr{
				slog.String("method", v.Method),
				slog.String("path", v.URIPath),
				slog.Int("status", v.Status),
				slog.Duration("latency", v.Latency),
				slog.String("request_id", v.RequestID),
			}
			level := slog.LevelInfo
			if v.Error != nil {
				attrs = append(attrs, slog.String("error", v.Error.Error()))
				if v.Status >= 500 {
					level = slog.LevelError
				}
			}
			log.LogAttrs(c.Request().Context(), level, "request", attrs...)
			return nil
		},
	})
}

func errorHandler(log *slog.Logger) echo.HTTPErrorHandler {
	return func(c *echo.Context, err error) {
		if r, uerr := echo.UnwrapResponse(c.Response()); uerr == nil && r.Committed {
			return
		}
		var app *apperror.Error
		var coder echo.HTTPStatusCoder
		if !errors.As(err, &app) && errors.As(err, &coder) {
			status := coder.StatusCode()
			code := apperror.CodeBadRequest
			switch {
			case status == http.StatusNotFound:
				code = apperror.CodeNotFound
			case status == http.StatusMethodNotAllowed:
				code = apperror.CodeMethodNotAllowed
			case status == http.StatusUnauthorized:
				code = apperror.CodeUnauthenticated
			case status >= 500:
				code = apperror.CodeInternal
			}
			detail := http.StatusText(status)
			var he *echo.HTTPError
			if errors.As(err, &he) && he.Message != "" {
				detail = he.Message
			}
			err = apperror.New(code, status, detail, err)
		}
		rid := c.Response().Header().Get(echo.HeaderXRequestID)
		p := apperror.ProblemFor(err, rid)
		if p.Status >= 500 {
			log.Error("unhandled error", slog.String("request_id", rid), slog.String("error", err.Error()))
		}
		c.Response().Header().Set(echo.HeaderContentType, "application/problem+json")
		if werr := c.JSON(p.Status, p); werr != nil {
			log.Error("write problem", slog.String("error", werr.Error()))
		}
	}
}

func spa(assets http.FileSystem) echo.HandlerFunc {
	files := http.FileServer(assets)
	return func(c *echo.Context) error {
		r := c.Request()
		if f, err := assets.Open(r.URL.Path); err == nil {
			_ = f.Close()
		} else {
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}
		files.ServeHTTP(c.Response(), r)
		return nil
	}
}
