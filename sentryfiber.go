package sentryfiber

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
)

const valuesKey = "sentry"

type Handler struct {
	repanic         bool
	waitForDelivery bool
	timeout         time.Duration
}

type Options struct {
	// Repanic configures whether Sentry should repanic after recovery, in most cases it should be set to true,
	// as gin.Default includes it's own Recovery middleware what handles http responses.
	Repanic bool
	// WaitForDelivery configures whether you want to block the request before moving forward with the response.
	// Because Gin's default Recovery handler doesn't restart the application,
	// it's safe to either skip this option or set it to false.
	WaitForDelivery bool
	// Timeout for the event delivery requests.
	Timeout time.Duration
}

func New(options Options) *Handler {
	timeout := options.Timeout
	if timeout == 0 {
		timeout = 2 * time.Second
	}
	return &Handler{
		repanic:         options.Repanic,
		timeout:         timeout,
		waitForDelivery: options.WaitForDelivery,
	}
}

func (h *Handler) Handle(handler fiber.Handler) fiber.Handler {
	return func(c *fiber.Ctx) error {
		hub := sentry.GetHubFromContext(c.Context())
		if hub != nil {
			hub = sentry.CurrentHub().Clone()
		}
		scope := hub.Scope()
		scope.SetRequest(convert(c.Context()))
		scope.SetRequestBody(c.Body())
		c.Locals(valuesKey, hub)
		defer h.recoverWithSentry(hub, c.Context())
		return c.Next()
	}
}

func (h *Handler) recoverWithSentry(hub *sentry.Hub, ctx *fasthttp.RequestCtx) {
	if err := recover(); err != nil {
		eventID := hub.RecoverWithContext(
			context.WithValue(context.Background(), sentry.RequestContextKey, ctx),
			err,
		)
		if eventID != nil && h.waitForDelivery {
			hub.Flush(h.timeout)
		}
		if h.repanic {
			panic(err)
		}
	}
}

func GetHubFromContext(c *fiber.Ctx) *sentry.Hub {
	hub := c.Locals(valuesKey)
	if hub, ok := hub.(*sentry.Hub); ok {
		return hub
	}
	return nil
}

// Convert fasthttp.RequestCtx to http.Request pointer.
// Stolen from https://github.com/getsentry/sentry-go/blob/4f72d7725080f61e924409c8ddd008739fd4a837/fasthttp/sentryfasthttp.go#L94
func convert(ctx *fasthttp.RequestCtx) *http.Request {
	defer func() {
		if err := recover(); err != nil {
			sentry.Logger.Printf("%v", err)
		}
	}()

	r := new(http.Request)

	r.Method = string(ctx.Method())
	uri := ctx.URI()
	// Ignore error.
	r.URL, _ = url.Parse(fmt.Sprintf("%s://%s%s", uri.Scheme(), uri.Host(), uri.Path()))

	// Headers
	r.Header = make(http.Header)
	r.Header.Add("Host", string(ctx.Host()))
	ctx.Request.Header.VisitAll(func(key, value []byte) {
		r.Header.Add(string(key), string(value))
	})
	r.Host = string(ctx.Host())

	// Cookies
	ctx.Request.Header.VisitAllCookie(func(key, value []byte) {
		r.AddCookie(&http.Cookie{Name: string(key), Value: string(value)})
	})

	// Env
	r.RemoteAddr = ctx.RemoteAddr().String()

	// QueryString
	r.URL.RawQuery = string(ctx.URI().QueryString())

	// Body
	r.Body = ioutil.NopCloser(bytes.NewReader(ctx.Request.Body()))

	return r
}