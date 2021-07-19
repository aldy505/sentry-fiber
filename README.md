# Fiber Handler for Sentry SDK

[![GitHub release (latest SemVer including pre-releases)](https://img.shields.io/github/v/release/aldy505/sentry-fiber?include_prereleases)](https://github.com/aldy505/sentry-fiber/releases) [![Go Reference](https://pkg.go.dev/badge/github.com/aldy505/sentry-fiber.svg)](https://pkg.go.dev/github.com/aldy505/sentry-fiber) [![Go Report Card](https://goreportcard.com/badge/github.com/aldy505/sentry-fiber)](https://goreportcard.com/report/github.com/aldy505/sentry-fiber) [![GitHub](https://img.shields.io/github/license/aldy505/sentry-fiber)](https://github.com/aldy505/sentry-fiber/blob/master/LICENSE) [![codecov](https://codecov.io/gh/aldy505/sentry-fiber/branch/master/graph/badge.svg?token=HUTQURBZ73)](https://codecov.io/gh/aldy505/sentry-fiber) [![CodeFactor](https://www.codefactor.io/repository/github/aldy505/sentry-fiber/badge)](https://www.codefactor.io/repository/github/aldy505/sentry-fiber) [![Codacy Badge](https://app.codacy.com/project/badge/Grade/16c40f49aabe4e89afea7c1e1d90a483)](https://www.codacy.com/gh/aldy505/sentry-fiber/dashboard?utm_source=github.com&amp;utm_medium=referral&amp;utm_content=aldy505/sentry-fiber&amp;utm_campaign=Badge_Grade) [![Build test](https://github.com/aldy505/sentry-fiber/actions/workflows/ci.yml/badge.svg)](https://github.com/aldy505/sentry-fiber/actions/workflows/ci.yml)

Welcome to another "I can't find one, so I made one" episode of mine. 

## Installation

```bash
$ go get github.com/aldy505/sentry-fiber
```
```go
import "github.com/aldy505/sentry-fiber"
```

## Usage

```go
import (
    "log"

    "github.com/gofiber/fiber/v2"
    "github.com/getsentry/sentry-go"
    sentryfiber "github.com/aldy505/sentry-fiber"
)

func main() {
  // Note that you'll need to have sentry-go in there.
  err := sentry.Init(sentry.ClientOptions{
    Dsn: "your-public-dsn",
  })
  if err != nil {
    log.Fatalln("sentry initialization failed")
  }

  app := fiber.New()
  app.Use(sentryfiber.New(sentryfiber.Options{}))

  app.Get("/", func (c *fiber.Ctx) error {
    return c.Send(fiber.StatusOK).Send([]byte("Hi there"))
  })

  app.Listen(":5000")
}
```

### Sentry Hub

sentryfiber attaches an instance of `*sentry.Hub` (https://godoc.org/github.com/getsentry/sentry-go#Hub) to the Request's Local context (`c.Locals()`), which makes it available throughout the rest of the request's lifetime.
You can access it by using the `sentryfiber.GetHubFromContext()` method on the context itself in any of your proceeding middleware and routes. 
And it should be used instead of the global `sentry.CaptureMessage`, `sentry.CaptureException`, or any other calls, as it keeps the separation of data between the requests.

Keep in mind that `*sentry.Hub` won't be available in middleware attached before to `sentryfiber`!

```go
app := fiber.New()

app.Use(sentryfiber.New(sentryfiber.Options{
    Repanic: true,
}))

app.Use(func() fiber.Handler {
  return func(c *fiber.Ctx) error {
    if hub := sentryfiber.GetHubFromContext(ctx); hub != nil {
        hub.Scope().SetTag("someRandomTag", "maybeYouNeedIt")
    }
    return c.Next()
  }
})

app.Get("/", func(c *fiber.Ctx) error {
    if hub := sentryfiber.GetHubFromContext(ctx); hub != nil {
        hub.WithScope(func(scope *sentry.Scope) {
            scope.SetExtra("unwantedQuery", "someQueryDataMaybe")
            hub.CaptureMessage("User provided unwanted query string, but we recovered just fine")
        })
    }
    return c.Status(fiber.StatusOK)
})

app.Get("/foo", func(c *fiber.Ctx) error {
    // sentryfiber handler will catch it just fine. Also, because we attached "someRandomTag"
    // in the middleware before, it will be sent through as well
    panic("y tho")
    return nil
})

app.Listen(":3000")
```


## Configuration

This is the configuration available for `sentryfiber.Options{}` struct.

```go
// Whether Sentry should repanic after recovery, in most cases it should be set to true,
// as gin.Default includes its own Recovery middleware that handles http responses.
Repanic         bool
// Whether you want to block the request before moving forward with the response.
// Because Gin's default `Recovery` handler doesn't restart the application,
// it's safe to either skip this option or set it to `false`.
WaitForDelivery bool
// Timeout for the event delivery requests.
Timeout         time.Duration
```

## Contribute

Yes please! I'm still new to Go and I create this module (or package if you will) to help me fulfill a need on my project. Feel free to refactor, add new feature, fix unknown bugs, and have fun!

## License

See [LICENSE](./LICENSE)