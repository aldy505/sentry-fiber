package sentryfiber_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	sentryfiber "github.com/aldy505/sentry-fiber"
	"github.com/getsentry/sentry-go"
	"github.com/gofiber/fiber/v2"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

type TestConfig struct {
	Path      string
	Method    string
	Body      string
	WantEvent *sentry.Event
}

func TestIntegration(t *testing.T) {
	largePayload := strings.Repeat("Large", 3*1024) // 15 KB

	tests := []TestConfig{
		{
			Path: "/panic",
			WantEvent: &sentry.Event{
				Level:   sentry.LevelFatal,
				Message: "test",
				Request: &sentry.Request{
					URL:    "http:///panic",
					Method: "GET",
					Headers: map[string]string{
						"Host": "",
					},
				},
			},
		},
		{
			Path:   "/post",
			Method: "POST",
			Body:   "payload",
			WantEvent: &sentry.Event{
				Level:   sentry.LevelInfo,
				Message: "post: payload",
				Request: &sentry.Request{
					URL:    "http:///post",
					Method: "POST",
					Data:   "payload",
					Headers: map[string]string{
						"Host": "",
					},
				},
			},
		},
		{
			Path: "/get",
			WantEvent: &sentry.Event{
				Level:   sentry.LevelInfo,
				Message: "get",
				Request: &sentry.Request{
					URL:    "http:///get",
					Method: "GET",
					Headers: map[string]string{
						"Host": "",
					},
				},
			},
		},
		{
			Path:   "/post/large",
			Method: "POST",
			Body:   largePayload,
			WantEvent: &sentry.Event{
				Level:   sentry.LevelInfo,
				Message: "post: 15 KB",
				Request: &sentry.Request{
					URL:    "http:///post/large",
					Method: "POST",
					// Actual request body omitted because too large.
					Data: "",
					Headers: map[string]string{
						"Host": "",
					},
				},
			},
		},
		{
			Path:   "/post/body-ignored",
			Method: "POST",
			Body:   "client sends, fasthttp always reads, SDK reports",
			WantEvent: &sentry.Event{
				Level:   sentry.LevelInfo,
				Message: "body ignored",
				Request: &sentry.Request{
					URL:    "http:///post/body-ignored",
					Method: "POST",
					// Actual request body included because fasthttp always
					// reads full request body.
					Data: "client sends, fasthttp always reads, SDK reports",
					Headers: map[string]string{
						"Host": "",
					},
				},
			},
		},
	}

	// Initialize Sentry
	eventsCh := make(chan *sentry.Event, len(tests))
	err := sentry.Init(sentry.ClientOptions{
		BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			eventsCh <- event
			return event
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var want []*sentry.Event
	app := App()
	for _, tt := range tests {
		want = append(want, tt.WantEvent)
		req, _ := http.NewRequest(tt.Method, tt.Path, strings.NewReader(tt.Body))
		res, err := app.Test(req, -1)
		if err != nil {
			t.Error(err)
		}
		if res.StatusCode != http.StatusOK {
			t.Errorf("Status code = %d", res.StatusCode)
		}
	}

	if ok := sentry.Flush(time.Second); !ok {
		t.Fatal("sentry.Flush timed out")
	}
	close(eventsCh)
	var got []*sentry.Event
	for e := range eventsCh {
		got = append(got, e)
	}
	opts := cmp.Options{
		cmpopts.IgnoreFields(
			sentry.Event{},
			"Contexts", "EventID", "Extra", "Platform",
			"Release", "Sdk", "ServerName", "Tags", "Timestamp",
		),
		cmpopts.IgnoreMapEntries(func(k string, v string) bool {
			// fasthttp changed Content-Length behavior in
			// https://github.com/valyala/fasthttp/commit/097fa05a697fc638624a14ab294f1336da9c29b0.
			// fasthttp changed Content-Type behavior in
			// https://github.com/valyala/fasthttp/commit/ffa0cabed8199819e372ebd2c739998914150ff2.
			// Since the specific values of those headers are not
			// important from the perspective of sentry-go, we
			// ignore them.
			return k == "Content-Length" || k == "Content-Type"
		}),
	}
	if diff := cmp.Diff(want, got, opts); diff != "" {
		t.Fatalf("Events mismatch (-want +got):\n%s", diff)
	}
}

func App() *fiber.App {
	// Initialize Fiber handler for Sentry
	sentryHandler := sentryfiber.New(sentryfiber.Options{})
	// Iniitalize Fiber app
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, e error) error {
			hub := sentryfiber.GetHubFromContext(c)
			hub.CaptureException(e)
			return nil
		},
	})

	app.Use(sentryHandler)

	app.Get("/panic", func(c *fiber.Ctx) error {
		panic("test")
	})

	app.Post("/post", func(c *fiber.Ctx) error {
		hub := sentryfiber.GetHubFromContext(c)
		hub.CaptureMessage("post: " + string(c.Body()))
		return nil
	})

	app.Get("/get", func(c *fiber.Ctx) error {
		hub := sentryfiber.GetHubFromContext(c)
		hub.CaptureMessage("get")
		return nil
	})

	app.Post("/post/large", func(c *fiber.Ctx) error {
		hub := sentryfiber.GetHubFromContext(c)
		hub.CaptureMessage(fmt.Sprintf("post: %d KB", len(c.Body())/1024))
		return nil
	})

	app.Post("/post/body-ignored", func(c *fiber.Ctx) error {
		hub := sentryfiber.GetHubFromContext(c)
		hub.CaptureMessage("body ignored")
		return nil
	})

	return app
}
