# WebSocket Route Coder Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add route-scoped binary WebSocket coding while preserving AQI's existing router and raw Text APIs.

**Architecture:** Text frames use the built-in JSON coder. A single optional custom binary coder adapts binary frames into the same `Request` pipeline. `Router.Coder(coder)` creates a coder-scoped `IRouter`; actions registered through that scope store the coder in route metadata. All `*Action` sends consult route metadata to choose Text/JSON or Binary/custom encoding, so no Binary-specific Send API is needed.

**Tech Stack:** Go, gobwas/ws, existing AQI ws package.

**Spec:** `docs/superpowers/specs/2026-09-14-ws-binary-coder-design.md`

## Global Constraints

- Keep one `IRouter`; do not add `IBinaryRouter`, `BinaryRouter`, or `AddBinary`.
- Use `Coder(Coder) IRouter` as the routing scope API.
- Keep `Client.Send chan []byte` and raw `SendMsg([]byte)` semantics as Text.
- Do not add `binarySend` or any Binary-specific Send family.
- Do not add protobuf/msgpack dependencies to AQI.
- Default Text handling remains built-in JSON with no registration.
- Keep one binary wire coder per AQI server in the first version.
- Replace exported `Dispatcher(c, string)` with internal `dispatcher(c *Client, req *Request)`.
- Keep `Context.Params string` for compatibility; do not duplicate it with a private `params []byte` field.
- PubSub remains unchanged.

---

### Task 1: Define generic Request and bidirectional Coder

**Files:**
- Create: `ws/request.go`
- Create: `ws/coder.go`
- Test: `ws/coder_test.go`

**Interfaces:**

```go
type Request struct {
    Id     string
    Action string
    Params []byte
}

type Coder interface {
    Decode([]byte) (*Request, error)
    Bind([]byte, any) error
    Encode(*Action) ([]byte, error)
}
```

- [x] Add JSON decode/bind coverage.
- [x] Add custom coder coverage.
- [x] Implement the built-in JSON coder and generic request contract.

### Task 2: Make Coder a Router scope

**Files:**
- Modify: `ws/router.go`
- Modify: `ws/manager.go`
- Test: `ws/coder_test.go`

**Interfaces:**

```go
type IRouter interface {
    Use(...HandlerFunc) IRouter
    Group(string) IRouter
    Coder(Coder) IRouter
    Add(string, ...HandlerFunc)
}
```

- [x] Store coder on the derived `Routers` value.
- [x] Store handlers + coder together as route metadata.
- [x] Keep the original router JSON/Text-only.
- [x] Register one shared binary decoder for inbound Binary frames.

### Task 3: Unify receive path around Request

**Files:**
- Modify: `ws/client.go`
- Modify: `ws/server_handler_http.go`
- Modify: `ws/dispatcher.go`
- Modify tests using `RequestQueue` / old `Dispatcher`.

- [x] Change `RequestQueue` to `chan *Request`.
- [x] Decode Text through built-in JSON and Binary through the registered coder.
- [x] Validate Text cannot target coder routes and Binary cannot target normal routes.
- [x] Route both through one internal `dispatcher(c, req)`.

### Task 4: Make Context binding route-aware

**Files:**
- Modify: `ws/context.go`
- Modify: `ws/context_binding.go`
- Test: `ws/coder_test.go`

- [x] Preserve public `Params string`.
- [x] Store only `request *Request` and route metadata internally.
- [x] Add `Bind(any) error` using the route coder or default JSON coder.
- [x] Keep existing `BindingJson` APIs intact.

### Task 5: Make existing Send APIs route-aware

**Files:**
- Modify: `ws/client.go`
- Modify: `ws/context_send.go`
- Modify: `ws/user.go`
- Modify: `ws/hubc.go`
- Test: `ws/coder_test.go`

- [x] Keep raw byte send APIs Text-only.
- [x] Reuse `frame{op,data}` and generalized `frameQueue` for Binary frames.
- [x] Remove `Client.SendBinary` and `Context.SendBinary`.
- [x] Encode all `*Action` sends from route metadata.
- [x] Apply route-aware encoding to current-client, user/app, and broadcast Context send methods.

### Task 6: Verification and cleanup

- [x] Compare branch against `main` and inspect changed files.
- [x] Remove stale `AddCoder`, `SendBinary`, and request-level coder concepts from production design.
- [ ] Run `go test ./ws/...` in an environment with repository/dependency access.
- [ ] Run `go test ./...` in an environment with repository/dependency access.
