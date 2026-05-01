# go-webview Agent Notes

This repository follows the v0.9.0 core/go contract:

- production code lives under `go/`
- package APIs return `core.Result` for recoverable outcomes
- direct banned stdlib imports are replaced by `dappco.re/go` primitives
- public symbols have Good, Bad, and Ugly tests plus examples in matching files

Do not edit `.core/`, `external/` submodule source, or `/Users/snider/Code/core/go`
when working on this repository.
