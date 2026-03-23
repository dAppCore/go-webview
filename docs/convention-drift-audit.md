# Convention Drift Audit

Date: 2026-03-23

Scope notes:
- `CLAUDE.md` reviewed.
- `CODEX.md` was not present anywhere under `/workspace`, so this audit is based on `CLAUDE.md` and the checked-in repository docs.
- `go test ./...` passes.
- `go test -coverprofile=webview.cover ./...` reports `16.1%` statement coverage.
- No source fixes were applied as part of this audit.

## `stdlib` -> `core.*`

- `docs/development.md:120` still tells contributors to wrap errors with `fmt.Errorf("context: %w", err)` so callers can use `errors.Is` and `errors.As`; `CLAUDE.md` now requires `coreerr.E("Scope.Method", "description", err)`. This is documentation drift rather than code drift.

## UK English

- `README.md:2` uses `License` in the badge alt text and badge label.
- `CONTRIBUTING.md:34` uses the US heading `License` instead of `Licence`.
- `docs/development.md:138` uses `licenced`; that is inconsistent with the repo's other licence/licensed wording.
- `webview.go:705` says `center coordinates` in a comment.
- `webview.go:718` says `center point` in a comment.
- `actions.go:511` says `center points` in a comment.

## Missing tests

- `actions.go:22`, `actions.go:33`, `actions.go:43`, `actions.go:74`, `actions.go:85`, `actions.go:97`, `actions.go:109`, `actions.go:121`, `actions.go:133`, `actions.go:153`, `actions.go:172`, `actions.go:189`, `actions.go:216`, `actions.go:263`, `actions.go:307`, `actions.go:378`, `actions.go:391`, `actions.go:404`, `actions.go:461`, `actions.go:471`, `actions.go:490` have no behavioural coverage. Existing action tests in `webview_test.go` only check field assignment and builder length, not execution paths.
- `angular.go:19`, `angular.go:27`, `angular.go:33`, `angular.go:41`, `angular.go:56`, `angular.go:93`, `angular.go:183`, `angular.go:214`, `angular.go:251`, `angular.go:331`, `angular.go:353`, `angular.go:384`, `angular.go:425`, `angular.go:453`, `angular.go:480`, `angular.go:517`, `angular.go:543`, `angular.go:570` are entirely uncovered. The Angular helper layer has no `_Good`, `_Bad`, or `_Ugly` behavioural tests.
- `cdp.go:78` is only lightly exercised by the invalid-debug-URL path; there is no success-path coverage for target discovery, tab creation, or WebSocket connection setup.
- `cdp.go:156`, `cdp.go:163`, `cdp.go:205`, `cdp.go:212`, `cdp.go:255`, `cdp.go:267`, `cdp.go:279`, `cdp.go:284`, `cdp.go:289`, `cdp.go:340`, `cdp.go:351`, `cdp.go:372`, `cdp.go:387` have no direct behavioural coverage for transport lifecycle, event dispatch, tab management, target enumeration, or version probing.
- `console.go:33`, `console.go:72`, `console.go:79`, `console.go:84`, `console.go:168`, `console.go:207`, `console.go:246`, `console.go:371`, `console.go:427`, `console.go:434`, `console.go:469` have no direct tests. The concurrency-sensitive watcher subscription, wait APIs, and event parsing paths are currently unverified.
- `webview.go:81` and `webview.go:110` are only partially covered; there is no success-path test for `WithDebugURL` plus `New` initialisation, including `Runtime.enable`, `Page.enable`, and `DOM.enable`.
- `webview.go:143`, `webview.go:152`, `webview.go:168`, `webview.go:176`, `webview.go:184`, `webview.go:192`, `webview.go:200`, `webview.go:219`, `webview.go:224`, `webview.go:238`, `webview.go:245`, `webview.go:272`, `webview.go:280`, `webview.go:288`, `webview.go:306`, `webview.go:324`, `webview.go:349`, `webview.go:363`, `webview.go:374`, `webview.go:387`, `webview.go:398`, `webview.go:422`, `webview.go:453`, `webview.go:495`, `webview.go:517`, `webview.go:541`, `webview.go:569`, `webview.go:604`, `webview.go:648`, `webview.go:704`, `webview.go:740` have no direct behavioural coverage across the main browser API, DOM lookup helpers, CDP evaluation path, and console capture path.

## SPDX headers

- `actions.go:1` is missing the required `// SPDX-License-Identifier: EUPL-1.2` header.
- `angular.go:1` is missing the required `// SPDX-License-Identifier: EUPL-1.2` header.
- `cdp.go:1` is missing the required `// SPDX-License-Identifier: EUPL-1.2` header.
- `console.go:1` is missing the required `// SPDX-License-Identifier: EUPL-1.2` header.
- `webview.go:1` is missing the required `// SPDX-License-Identifier: EUPL-1.2` header.
- `webview_test.go:1` is missing the required `// SPDX-License-Identifier: EUPL-1.2` header.
