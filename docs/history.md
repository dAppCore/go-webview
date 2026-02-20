# Project History

## Origin

go-webview was extracted from the `pkg/webview/` directory of `forge.lthn.ai/core/go` on 19 February 2026 by Virgil. The extraction made the package independently importable and gave it its own module path, dependency management, and commit history.

---

## Completed Phases

### Extraction from core/go

Commit `45f119b9ac0e0ebe34f5c8387a070a5b8bd2de6b` — 2026-02-19

Initial extraction. All source files were moved from `go` `pkg/webview/` into the root of this repository. The module was renamed from the internal path to `forge.lthn.ai/core/go-webview`.

Files established at extraction:

- `webview.go` — `Webview` struct, public API
- `cdp.go` — `CDPClient` WebSocket transport
- `actions.go` — `Action` interface and all concrete action types
- `console.go` — `ConsoleWatcher` and `ExceptionWatcher`
- `angular.go` — `AngularHelper`
- `webview_test.go` — struct and option tests

### Dependency Fix

Commit `56d2d476a110b740828ff66b44cc4cbd689dd969` — 2026-02-19

Added `github.com/gorilla/websocket v1.5.3` to `go.mod` and `go.sum`. The `go.mod` was missing the explicit `require` directive after the extraction.

### Fleet Delegation Docs

Commit `b752f155459c8bf610cfcc6486f4a9431ec4b7c3` — 2026-02-19

Added `TODO.md` and `FINDINGS.md` to record research findings and task backlog for the agent fleet. These files have since been superseded by this `docs/` directory and have been removed.

---

## Known Limitations

### No Chrome Launcher

The package has no built-in mechanism for starting Chrome. All usage requires the caller to ensure a Chrome or Chromium process is already running with `--remote-debugging-port=9222`. This is intentional — launcher behaviour (headless vs headed, profile selection, proxy configuration) varies too widely between use cases to be handled generically.

### Single-Tab Webview

The `Webview` struct connects to one tab at construction time. `CDPClient.NewTab` returns a raw `CDPClient` rather than a full `Webview`, so the high-level API (navigate, click, screenshot, etc.) is not available directly on new tabs without wrapping the client manually.

### Console Capture: Value-Only Arguments

The console event handler extracts only arguments with a `value` field (primitives). Complex objects that Chrome serialises as `type: "object"` with a `description` rather than a `value` are silently omitted from the captured text. This means `console.log({foo: 'bar'})` will produce an empty message.

### Angular Helper: Debug Mode Dependency

`AngularHelper` methods that use `window.ng.probe` require the Angular application to be running in development mode (debug mode). Production builds compiled with `enableProdMode()` disable the debug utilities, making component introspection, `GetComponentProperty`, `SetComponentProperty`, `CallComponentMethod`, and `GetService` non-functional. `WaitForAngular` has a Zone.js polling fallback that works in production.

### Angular Helper: AngularJS 1.x

Detection of AngularJS 1.x (`window.angular.element`) is implemented in `isAngularApp`, but none of the other `AngularHelper` methods have 1.x-specific code paths. Methods beyond `WaitForAngular` are Angular 2+ only.

### Zone Stability Timeout

The JavaScript Promise used in `waitForZoneStability` has an internal 5-second `setTimeout` fallback. If Zone.js does not become stable within that window, the Promise resolves with the current stability value rather than rejecting. This can cause `WaitForAngular` to return successfully even when the application is still processing asynchronous work.

### waitForLoad Polling

`waitForLoad` polls `document.readyState` at 100 ms intervals rather than subscribing to the `Page.loadEventFired` CDP event. This works correctly but adds up to 100 ms of unnecessary latency after the load event fires.

### Key Dispatch: Modifier Keys

`PressKeyAction` handles named keys but does not support modifier combinations (Ctrl+A, Shift+Tab, etc.). Modifier key presses require constructing the CDP params map manually and calling `CDPClient.Call` directly.

### CloseTab Implementation

`CDPClient.CloseTab` calls `Browser.close`, which closes the entire browser rather than just the tab. The correct CDP command for closing a single tab is `Target.closeTarget` with the target's ID extracted from the WebSocket URL. This is a bug.

---

## Future Considerations

The following were identified as planned phases at the time of extraction. They are recorded here for context but are not committed work items.

- **Headless CI tests** — integration tests using `net/http/httptest` fixtures and Chrome with `--headless=new`, runnable without a display.
- **Multi-tab lifecycle tests** — tests verifying that `NewTab`/`CloseTab` work correctly and that console events are scoped to their tab.
- **Angular helper expansion** — component detection by Angular selector, proper production-mode router integration.
- **Visual regression** — pixel-diff comparison between a baseline screenshot and a current screenshot, with configurable threshold and diff image output.
