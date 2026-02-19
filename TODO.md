# TODO.md -- go-webview

## Phase 1: Test Coverage

- [ ] Add headless Chrome tests runnable in CI (use `--headless=new` flag)
- [ ] Test navigation, click, type, screenshot in headless mode
- [ ] Add test fixtures (minimal HTML pages served by httptest)

## Phase 2: Multi-Tab Support

- [ ] `NewTab` / `CloseTab` exist but are not well tested
- [ ] Add tests for multi-tab lifecycle (open, switch, close)
- [ ] Verify console capture is tab-scoped (no cross-tab bleed)

## Phase 3: Angular Helpers

- [ ] Expand Angular-specific helpers beyond current set
- [ ] Add component detection (query by Angular selector)
- [ ] Add router integration (wait for navigation, read current route)
- [ ] Add zone.js stability detection (wait for async operations)

## Phase 4: Screenshot Comparison

- [ ] Add visual regression testing support
- [ ] Pixel-diff between baseline and current screenshots
- [ ] Configurable threshold for acceptable difference percentage
- [ ] Save diff images for failed comparisons

---

## Workflow

1. Virgil in core/go writes tasks here after research
2. This repo's dedicated session picks up tasks in phase order
3. Mark `[x]` when done, note commit hash
