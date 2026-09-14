# WebSocket Route Coder Design

## Goal

Add first-class binary WebSocket support to AQI without introducing a second router, binary-specific dispatcher, binary send API, or changing `Client.Send chan []byte` text semantics.

## Core model

AQI keeps one `IRouter`, one action namespace, one dispatcher, and one generic request shape:

```go
type Request struct {
    Id     string
    Action string
    Params []byte
}
```

Text frames use AQI's built-in JSON coder. Binary frames use one registered custom coder. After decode, both paths enqueue `*Request` and enter the same dispatcher and middleware chain.

## Route-scoped coder

`Coder` is a router scope, like `Group` and `Use`:

```go
type IRouter interface {
    Use(...HandlerFunc) IRouter
    Group(string) IRouter
    Coder(Coder) IRouter
    Add(string, ...HandlerFunc)
}
```

Usage:

```go
func (a *Actions) Register(router ws.IRouter) {
    router.Add("arcade.login", a.login)
    router.Add("room.create", a.createRoom)

    binary := router.Coder(protoCoder)
    binary.Add("dungeon.input", a.dungeonInput)
    binary.Add("dungeon.state", a.dungeonState)
}
```

`Coder(...)` returns another `IRouter` value carrying the coder scope. Routes registered through that value store the coder as route metadata. The original router remains a normal JSON/Text scope.

AQI supports one binary wire coder per server in the first version, while any number of actions can be registered inside that coder scope.

## Coder contract

```go
type Coder interface {
    Decode([]byte) (*Request, error)
    Bind([]byte, any) error
    Encode(*Action) ([]byte, error)
}
```

- `Decode` adapts an inbound binary frame into AQI's generic `Request`.
- `Bind` decodes `Request.Params` into the handler's request type.
- `Encode` converts an AQI `Action` into the binary wire format.

The built-in JSON coder implements the same contract internally and requires no registration.

## Receive path

```text
OpText   -> built-in JSON Decode ----\
                                      -> RequestQueue -> dispatcher -> route -> handler
OpBinary -> registered Coder Decode -/
```

`RequestQueue` is `chan *Request` and `dispatcher` is internal:

```go
func dispatcher(c *Client, req *Request)
```

Reader validates the frame against route metadata: text frames cannot target coder routes and binary frames cannot target normal text routes.

## Context binding

`Context.Params string` remains for compatibility with existing JSON handlers and templates. `Context` also keeps the current internal `request` and `route` references.

```go
func (c *Context) Bind(v any) error
```

`Bind` uses the route coder when present and otherwise uses the built-in JSON coder. Existing `BindingJson` APIs remain unchanged.

## Send path

Business APIs never choose Text vs Binary explicitly. There is no `SendBinary`, `SendToBinary`, or `BroadcastBinary` family.

All `*Action` sends are encoded from route metadata:

```text
route.coder == nil -> JSON Encode -> OpText
route.coder != nil -> coder.Encode -> OpBinary
```

This applies to `Send`, `SendOk`, `SendCode`, `SendAction`, `SendActionData`, `SendActionMsg`, `SendTo`, `SendToApp`, `SendToApps`, `SendRawTo`, and `Broadcast`.

Raw byte APIs retain their existing meaning:

```text
Client.Send chan []byte -> Text
Client.SendMsg([]byte)   -> Text
User.SendMsg([]byte)     -> Text
Hub.Broadcast([]byte)    -> Text
```

The existing `frame{op,data}` abstraction is reused. `controlQueue` is generalized to `frameQueue` for control and non-text application frames; no `binarySend` channel is added.

## Non-goals

- No `IBinaryRouter` / `BinaryRouter`.
- No `AddBinary` or Binary-specific Send APIs.
- No second action manager or dispatcher.
- No protobuf/msgpack dependency inside AQI.
- No `Client.coder`; a single connection may carry both JSON control-plane actions and binary realtime actions.
- No multiple simultaneous binary wire coders in the first version.
- PubSub encoding is unchanged in this change.
