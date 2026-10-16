package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	"github.com/Pleasurecruise/eyeful/internal/apperror"
)

type Handler interface {
	Register(g *echo.Group)
}

type Server struct {
	echo *echo.Echo
	log  *slog.Logger
}

func New(log *slog.Logger, handlers ...Handler) *Server {
	e := echo.New()
	e.Logger = log
	e.HTTPErrorHandler = errorHandler(log)
	e.Use(middleware.RequestID())
	e.Use(middleware.Recover())
	e.Use(accessLog(log))
	e.Use(middleware.BodyLimit(5 << 20))
	// TODO(observability): OpenTelemetry tracing, one span per request and per review stage.

	g := e.Group("")
	for _, h := range handlers {
		h.Register(g)
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
