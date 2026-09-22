# AQI Cluster Pub/Sub Design

## Goal

Add opt-in multi-node realtime message routing to AQI while preserving existing single-node behavior.

The default path stays simple:

```go
aqi.Init(
    aqi.WithCluster(),
)
```

`WithCluster()` uses AQI's built-in Redis Pub/Sub transport and resolves the existing named Redis configuration at `redis.aqi` after configuration has loaded.

AQI also exposes one minimal escape hatch for applications that need another inter-node transport:

```go
aqi.Init(
    aqi.WithClusterTransport(func(handler aqi.ClusterMessageHandler) (aqi.ClusterTransport, error) {
        return newMyTransport(handler)
    }),
)
```

A custom transport replaces the built-in Redis transport; it does not add another messaging layer on top of Redis.

## Non-goals / hard boundaries

Cluster does **not** provide:

- reliable message storage, ACK, retry, replay, or ordering guarantees
- Room/Game/business-state synchronization
- distributed locks
- global user/session state persistence
- cross-node restoration of `User.SubTopics`
- authoritative global presence
- user-to-node registries
- node failover/state migration
- a generic MQ abstraction for application features

Reliable storage remains a business-layer decision.

## Existing AQI semantics to preserve

- `Client` is a physical WebSocket connection.
- `User` owns one or more local `AppClients`.
- `Topic` subscriptions are user-based (`Topic.SubUsers`), not client-based.
- Users and their `SubTopics` remain in local memory for about five minutes after their last client disconnects to support short interruptions such as phone calls/backgrounding.
- Existing in-process `PubSub.Pub` remains best-effort and local; cluster support must not silently redefine all current PubSub semantics.

## Default Redis configuration

AQI already supports named Redis stores. The built-in Cluster transport reuses that mechanism and reserves:

```yaml
redis:
  aqi:
    addr: 127.0.0.1:6379
    username: ""
    pwd: ""
    db: 0
```

`WithCluster()` only enables the capability. Redis is resolved after YAML is loaded. If `redis.aqi` is absent or unusable, startup fails clearly.

`WithClusterTransport(...)` bypasses `redis.aqi`; configuration and construction of that custom transport belong to its factory.

No configured node identifier is required by either path.

## Cluster transport contract

AQI's transport surface is deliberately small and public:

```go
type ClusterTransport interface {
    Subscribe(channel string) error
    Unsubscribe(channel string) error
    Publish(channel string, data []byte) error
    Close() error
}

type ClusterMessageHandler func(channel string, data []byte)

type ClusterTransportFactory func(ClusterMessageHandler) (ClusterTransport, error)
```

`Subscribe` and `Unsubscribe` are idempotent desired-state operations. An open transport must record the latest requested membership before returning, even when it returns an error. Errors report an immediate backend failure; they do not discard the requested state. The transport owns reconciliation after recovery without requiring further AQI calls. A newer request supersedes older pending operations, including an unsubscribe followed by a subscribe during an outage. Implementations must support concurrent calls for different channels. `Close` stops reconciliation and releases subscriptions; calls after close may fail.

AQI tracks local owners separately from whether a subscribe call succeeded. It may repeat subscription calls during later local transitions, but does not run a subscription retry worker. Once the final owner leaves, AQI sends the unsubscribe intent and deletes its local channel record regardless of the returned error. Custom transports that only attempt a network command and discard failed intent must be adapted to this contract.

The factory is invoked after AQI configuration has loaded. AQI supplies the inbound `ClusterMessageHandler`; a custom transport invokes that handler whenever it receives a message from the underlying transport.

The transport treats both `channel` and `data` as opaque AQI values:

- it must preserve channel identity
- it must preserve payload bytes
- it must not decode AQI users, topics, actions, or wire payloads
- it must not transport `*Client`, `*User`, `TopicMsg.Ori`, Hub state, or arbitrary Go objects

A backend adapter may encode the logical AQI channel into a backend-specific physical name when necessary, but it must decode it back before invoking `ClusterMessageHandler`. This allows transports with stricter naming rules without imposing those rules on AQI business Topic IDs.

Cluster transport installation is startup-only. AQI accepts one transport for the process lifetime and rejects a second initialization instead of hot-swapping transports at runtime.

Redis Pub/Sub is the first built-in implementation. Future transports should implement this same minimal contract rather than expanding Cluster into a general messaging framework.

## AQI channel namespace

AQI owns two logical cluster channel namespaces:

```text
$aqi:user:<uid>
$aqi:topic:<topicId>
```

Business code never needs to construct these names. It continues to use ordinary user IDs and Topic IDs:

```text
B          -> $aqi:user:B
room:123   -> $aqi:topic:room:123
game:456   -> $aqi:topic:game:456
```

AQI does not interpret the business Topic ID. `room`, `game`, `chat`, or any other prefix belongs to the business layer.

Business Topic IDs are not restricted. A business may even use a Topic ID such as `$aqi:user:B`; AQI maps it to `$aqi:topic:$aqi:user:B`, which remains distinct from the internal direct-user channel `$aqi:user:B`.

The Redis implementation uses these logical channel names directly. Other transports may encode them internally if their backend requires it.

## User routing

Each logged-in user has an internal cluster channel derived from the user ID, for example:

```text
$aqi:user:B
```

AQI subscribes at **node level**, not per physical client.

For a local user B:

- `AppClients` changes `0 -> 1`: acquire `$aqi:user:B`
- `AppClients` changes `1 -> 2` (or more): no transport change
- `AppClients` changes `2 -> 1`: no transport change
- `AppClients` changes `1 -> 0`: release `$aqi:user:B`

Thus, if B has five devices spread across five AQI nodes, at most five AQI nodes subscribe to `$aqi:user:B`. If several devices land on one node, that node still has only one transport subscription.

A cross-node direct send publishes to `$aqi:user:B`. Only AQI nodes currently subscribed to that user receive it. Each receiving node then sends to its local `User.AppClients`.

No `user -> node` registry is maintained.

## Topic routing

Business Topic IDs are wrapped only at the Cluster boundary:

```text
room:123 -> $aqi:topic:room:123
```

For normal topics, Cluster maintains a node-local online reference count for the logical channel:

```text
clusterRefs[$aqi:topic:<topicId>] = number of locally-online subscribed users
```

Transport transitions only on the edges:

- `0 -> 1`: `Subscribe($aqi:topic:<topicId>)`
- `1 -> 0`: `Unsubscribe($aqi:topic:<topicId>)`

Multiple local users subscribed to the same business topic still create only one transport subscription from that AQI node.

On inbound delivery, AQI strips `$aqi:topic:` and delivers the original Topic ID to the local PubSub layer.

## Five-minute reconnect grace period

The existing five-minute local `User`/`SubTopics` retention remains unchanged.

Important distinction:

- local subscription state may remain in `User.SubTopics` during the grace period
- Cluster subscriptions represent **currently online local users only**

When a user's last local client disconnects:

1. release the user's `$aqi:user:<id>` channel
2. release `$aqi:topic:<topicId>` refs for that user's existing `SubTopics`
3. keep `User` and `SubTopics` locally as today

If the user reconnects to the **same node** during the grace period:

1. reacquire `$aqi:user:<id>`
2. reacquire `$aqi:topic:<topicId>` refs for the retained `SubTopics`

If the user reconnects to a **different node**, only identity routing is re-established there. AQI does not synchronize the old node's `SubTopics`; the business/client must resubscribe as needed. This is intentional to avoid turning Cluster into distributed session storage.

Expiry cleanup and login for the same UID are serialized at the Hub lifecycle boundary. Cleanup revalidates that the same `User` is still mapped, still offline, and still older than the five-minute cutoff before deleting it. This prevents a reconnect from being removed by a stale cleanup decision while preserving the existing grace period.

## Publish path and loop prevention

Local delivery does not depend on the Cluster transport.

For a cluster-aware publish:

1. deliver locally
2. publish the same encoded AQI payload through the Cluster transport for remote nodes

Transport-delivered messages are injected into a local-only delivery path and must never be republished to Cluster.

Each AQI process generates a random 16-byte `instanceID` when the Cluster transport is installed. The wire payload is deliberately minimal:

```text
[16-byte instanceID][payload]
```

The `instanceID` exists only to drop transport self-echo. It is not a configured node identity, is not stable across restarts, and is not used for ownership or routing. No protocol version or additional envelope fields are added unless a concrete future need appears.

A transport is allowed not to echo a publisher's own message; the marker still remains harmless. If the backend does self-echo, AQI drops the local echo by `instanceID`.

## Lifecycle integration

Cluster integration attaches to existing AQI lifecycle boundaries rather than `Client` socket hooks directly:

- login transition: `User.AppClients` `0 -> 1`
- logout transition: `User.AppClients` `1 -> 0`
- topic add/remove for online users

The same-app replacement path must not cause false `1 -> 0 -> 1` transitions when the old client disconnects after being replaced.

Subscription membership and the online snapshot used for Cluster reference acquisition are read under the same `User` lock so concurrent login/subscription changes cannot double-acquire a topic reference.

Per-channel synchronization reconciles each user’s ownership against current local membership and online state. Delayed reconnect or unsubscribe operations cannot release another user’s ownership.

## Failure semantics

Cluster is realtime best-effort transport.

- If the active transport is temporarily unavailable, cross-node delivery can fail.
- Local delivery should continue where possible.
- Offline/reconnect gaps can lose realtime messages.
- AQI does not replay missed messages.
- Transports must retain and reconcile subscription intent after recovery. This does not guarantee message delivery or require buffering/replaying published messages.

Businesses that require reliable delivery must persist and reconcile messages themselves.

The built-in Redis transport is validated at startup with a bounded Ping. A custom factory is responsible for returning a useful initialization error if its transport cannot start.

## Tests

Minimum coverage:

1. Cluster disabled preserves current behavior and requires no transport configuration.
2. `WithCluster()` fails startup when `redis.aqi` is missing/invalid.
3. `WithClusterTransport(...)` is publicly implementable outside the `aqi` package and bypasses `redis.aqi`.
4. A nil custom factory is rejected.
5. Cluster transport can only be initialized once; a second initialization cannot replace or close the active transport.
6. First local login subscribes `$aqi:user:<id>` exactly once.
7. Additional local clients for the same user do not resubscribe.
8. Last local client disconnect unsubscribes exactly once.
9. Same-app client replacement does not accidentally unsubscribe an online user.
10. A retained user's topics are released while offline and reacquired on same-node reconnect.
11. First online local subscriber to a normal topic acquires `$aqi:topic:<topicId>`; last online subscriber releases it.
12. Business Topic IDs beginning with `$aqi:` remain valid and are nested under `$aqi:topic:`.
13. Cross-node user send reaches the correct remote node only.
14. Cross-node topic publish reaches subscribed nodes and not unrelated nodes.
15. Transport-originated messages are delivered locally without republishing (no loop).
16. Topic reference transitions remain ordered when Subscribe is slow and Release races it.
17. Reconnect racing an unsubscribe cannot leave a ghost topic reference.
18. Topic subscribe racing login uses one atomic user online snapshot and cannot double-acquire.
19. User login racing five-minute expiry cleanup cannot lose the reconnected user.
20. Race tests pass for login/logout/topic/Cluster lifecycle.

## Scope guard

If implementation starts requiring any of the following, stop instead of expanding scope:

- Redis keys or transport metadata for user/node registries
- TTL/heartbeat-based node ownership
- Streams/queues/ACK/retry
- distributed session/topic persistence
- Room/Game state movement
- distributed locks
- generic application-level request/reply, queue groups, streams, consumers, or backend-specific messaging features

Those are separate features, not part of AQI Cluster.
