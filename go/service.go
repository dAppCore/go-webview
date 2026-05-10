// SPDX-License-Identifier: EUPL-1.2

// Service registration for the webview package — exposes the canonical
// `NewService(opts)` + `Register(c)` shape per Mantis #1336, wrapping
// the existing `New(opts ...Option) (*Webview, error)` constructor in
// a Core-registerable factory.
//
//	c, _ := core.New(
//	    core.WithService(webview.NewService(webview.ServiceOptions{
//	        DebugURL:     "http://localhost:9222",
//	        Timeout:      30 * time.Second,
//	        ConsoleLimit: 1000,
//	    })),
//	)
//	svc := core.MustServiceFor[*webview.Service](c, "webview")
//	r := svc.Webview.Click(ctx, ".btn")
//
// The *Webview type and the New(opts...) constructor remain the source
// of truth — Service is a thin Core-side handle that holds the wired
// *Webview alongside typed options access.

package webview

import (
	"time"

	core "dappco.re/go"
)

// ServiceOptions configures the webview service. Empty values fall
// back to the package's New() defaults (no DebugURL → caller must
// supply via WithDebugURL or wire CDP elsewhere; 30s Timeout; 1000
// ConsoleLimit).
//
// Named ServiceOptions (not Options) to avoid colliding with the
// existing `type Option func(*Webview) error` from webview.go.
type ServiceOptions struct {
	// DebugURL is the Chrome DevTools Protocol endpoint.
	// Empty → no upstream wired; Service.Webview.Connect() before use.
	DebugURL string
	// Timeout is the per-CDP-call timeout. Zero → 30s.
	Timeout time.Duration
	// ConsoleLimit caps the captured console-log buffer. Zero → 1000.
	ConsoleLimit int
}

// Service is the registerable handle for the webview package — embeds
// *core.ServiceRuntime[ServiceOptions] for typed options access and
// holds a constructed *Webview ready for direct method calls.
//
// Usage example: `svc := core.MustServiceFor[*webview.Service](c, "webview"); _ = svc.Webview.Click(ctx, ".btn")`
type Service struct {
	*core.ServiceRuntime[ServiceOptions]
	// Webview is the live *Webview the service was constructed with.
	// nil if NewService was called with empty ServiceOptions.DebugURL —
	// callers can construct one later via webview.New(...) and assign
	// to svc.Webview.
	Webview *Webview
}

// NewService returns a factory that constructs a *Webview from the
// supplied options and wraps it as a Core-registerable *Service. If
// ServiceOptions.DebugURL is empty, the service is registered with a
// nil Webview — callers that wire CDP later (e.g. on first Connect)
// can mutate svc.Webview themselves.
//
//	core.WithService(webview.NewService(webview.ServiceOptions{
//	    DebugURL: "http://localhost:9222",
//	}))
func NewService(opts ServiceOptions) func(*core.Core) core.Result {
	return func(c *core.Core) core.Result {
		svc := &Service{
			ServiceRuntime: core.NewServiceRuntime(c, opts),
		}
		if opts.DebugURL == "" {
			return core.Ok(svc)
		}
		webviewOpts := []Option{WithDebugURL(opts.DebugURL)}
		if opts.Timeout > 0 {
			webviewOpts = append(webviewOpts, WithTimeout(opts.Timeout))
		}
		if opts.ConsoleLimit > 0 {
			webviewOpts = append(webviewOpts, WithConsoleLimit(opts.ConsoleLimit))
		}
		wv, err := New(webviewOpts...)
		if err != nil {
			return core.Fail(core.E("webview.NewService", "webview construction failed", err))
		}
		svc.Webview = wv
		return core.Ok(svc)
	}
}

// Register wires the webview service into the Core with empty
// ServiceOptions — the imperative-style alternative to NewService.
// The resulting *Service holds a nil Webview, so consumers must wire
// one via webview.New(...) and assign to svc.Webview before use.
//
//	c := core.New()
//	if r := webview.Register(c); !r.OK { return r }
//	svc := core.MustServiceFor[*webview.Service](c, "webview")
//	wv, _ := webview.New(webview.WithDebugURL("http://localhost:9222"))
//	svc.Webview = wv
func Register(c *core.Core) core.Result {
	return NewService(ServiceOptions{})(c)
}
