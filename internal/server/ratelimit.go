package server

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/Pleasurecruise/eyeful/internal/apperror"
)

type window struct {
	start time.Time
	count int
}

// TODO(scheduling): Redis-backed fixed window when several servers share one address.
func RateLimit(mark string, limit int, per time.Duration) echo.MiddlewareFunc {
	var mu sync.Mutex
	windows := map[string]window{}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if limit <= 0 || c.Path() == consoleRoute {
				return next(c)
			}
			now := time.Now()
			key := mark + ":" + c.RealIP()
			mu.Lock()
			if len(windows) > 10_000 {
				for k, w := range windows {
					if now.Sub(w.start) >= per {
						delete(windows, k)
					}
				}
			}
			w := windows[key]
			if now.Sub(w.start) >= per {
				w = window{start: now}
			}
			w.count++
			windows[key] = w
			mu.Unlock()
			if w.count > limit {
				retry := max(int(w.start.Add(per).Sub(now).Seconds()), 1)
				c.Response().Header().Set("Retry-After", strconv.Itoa(retry))
				return apperror.New(apperror.CodeRateLimited, http.StatusTooManyRequests, "too many requests; retry after "+strconv.Itoa(retry)+"s", nil)
			}
			return next(c)
		}
	}
}
