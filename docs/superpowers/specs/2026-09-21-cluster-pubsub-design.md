# AQI Cluster Pub/Sub Design

## Goal

Add opt-in multi-node realtime message routing to AQI with a single startup option while preserving existing single-node behavior.

```go
aqi.Init(
    aqi.WithCluster(),
)
```

When enabled, AQI requires the existing named Redis configuration at `redis.aqi` and uses Redis Pub/Sub only as a realtime inter-node transport.

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

Reliable storage remains a business-layer decision.

## Existing AQI semantics to preserve

- `Client` is a physical WebSocket connection.
- `User` owns one or more local `AppClients`.
- `Topic` subscriptions are user-based (`Topic.SubUsers`), not client-based.
- Users and their `SubTopics` remain in local memory for about five minutes after their last client disconnects to support short interruptions such as phone calls/backgrounding.
- Existing in-process `PubSub.Pub` remains best-effort and local; cluster support must not silently redefine all current PubSub semantics.

## Redis configuration

AQI already supports named Redis stores. Cluster reuses that mechanism and reserves:

```yaml
redis:
  aqi:
    addr: 127.0.0.1:6379
    username: ""
    pwd: ""
    db: 0
```

`WithCluster()` only enables the feature. Redis is resolved after YAML is loaded. If `redis.aqi` is absent or unusable, startup fails clearly.

No Redis address or node identifier is passed to `WithCluster()`.

## Cluster transport

The internal transport surface stays deliberately small:

```go
type Cluster interface {
    Subscribe(topic string) error
    Unsubscribe(topic string) error
    Publish(topic string, data []byte) error
    Close() error
}
```

The first implementation uses Redis Pub/Sub.

Cluster transports bytes only. It never transports `*Client`, `*User`, `TopicMsg.Ori`, Hub state, or arbitrary Go objects.

## Redis channel namespace

AQI owns two internal Redis channel namespaces:

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

## User routing

Each logged-in user has an internal cluster channel derived from the user ID, for example:

```text
$aqi:user:B
```

AQI subscribes at **node level**, not per physical client.

For a local user B:

- `AppClients` changes `0 -> 1`: subscribe `$aqi:user:B`
- `AppClients` changes `1 -> 2` (or more): no Redis change
- `AppClients` changes `2 -> 1`: no Redis change
- `AppClients` changes `1 -> 0`: unsubscribe `$aqi:user:B`

Thus, if B has five devices spread across five AQI nodes, Redis has at most five subscribers for `$aqi:user:B`. If several devices land on one node, that node still has only one Redis subscription.

A cross-node direct send publishes to `$aqi:user:B`. Redis delivers only to AQI nodes currently subscribed to that user. Each receiving node then sends to its local `User.AppClients`.

No `user -> node` registry is maintained.

## Topic routing

Business Topic IDs are wrapped only at the Redis boundary:

```text
room:123 -> $aqi:topic:room:123
```

For normal topics, Cluster maintains a node-local online reference count for the internal Redis channel:

```text
clusterRefs[$aqi:topic:<topicId>] = number of locally-online subscribed users
```

Redis transitions only on the edges:

- `0 -> 1`: `SUBSCRIBE $aqi:topic:<topicId>`
- `1 -> 0`: `UNSUBSCRIBE $aqi:topic:<topicId>`

Multiple local users subscribed to the same business topic still create only one Redis subscription from that AQI node.

On inbound Redis delivery, AQI strips `$aqi:topic:` and delivers the original Topic ID to the local PubSub layer.

## Five-minute reconnect grace period

The existing five-minute local `User`/`SubTopics` retention remains unchanged.

Important distinction:

- local subscription state may remain in `User.SubTopics` during the grace period
- Redis subscriptions represent **currently online local users only**

When a user's last local client disconnects:

1. unsubscribe the user's `$aqi:user:<id>` cluster channel
2. release `$aqi:topic:<topicId>` cluster refs for that user's existing `SubTopics`
3. keep `User` and `SubTopics` locally as today

If the user reconnects to the **same node** during the grace period:

1. subscribe `$aqi:user:<id>` again
2. reacquire `$aqi:topic:<topicId>` cluster refs for the retained `SubTopics`

If the user reconnects to a **different node**, only identity routing is re-established there. AQI does not synchronize the old node's `SubTopics`; the business/client must resubscribe as needed. This is intentional to avoid turning Cluster into distributed session storage.

## Publish path and loop prevention

Local delivery should not depend on Redis.

For a cluster-aware publish:

1. deliver locally
2. publish encoded bytes to Redis for remote nodes

Redis-delivered messages are injected into a local-only delivery path and must never be republished to Redis.

Each AQI process generates a random 16-byte `instanceID` when cluster transport is installed. The Redis wire payload is deliberately minimal:

```text
[16-byte instanceID][payload]
```

The `instanceID` exists only to drop Redis self-echo. It is not a configured node identity, is not stable across restarts, and is not used for ownership or routing. No protocol version or additional envelope fields are added unless a concrete future need appears.

## Lifecycle integration

Cluster integration should attach to existing AQI lifecycle boundaries rather than `Client` socket hooks directly:

- login transition: `User.AppClients` `0 -> 1`
- logout transition: `User.AppClients` `1 -> 0`
- topic add/remove for online users

The same-app replacement path must not cause false `1 -> 0 -> 1` transitions when the old client disconnects after being replaced.

## Existing unsubscribe cleanup issue

Current `PubSub.Unsub()` and `User.UnsubTopic()` both lead to `Topic.RemoveSubUser()`. This is harmless today because `sync.Map.Delete` is idempotent, but cluster reference counting would make double-release incorrect.

Before wiring cluster refs into subscription lifecycle, subscription removal must be reduced to one authoritative path.

## Failure semantics

Redis Pub/Sub is realtime best-effort transport.

- If Redis is temporarily unavailable, cross-node delivery can fail.
- Local delivery should continue where possible.
- Offline/reconnect gaps can lose realtime messages.
- AQI does not replay missed messages.

Businesses that require reliable delivery must persist and reconcile messages themselves.

## Tests

Minimum coverage:

1. `WithCluster()` disabled preserves current behavior and requires no Redis.
2. `WithCluster()` enabled fails startup when `redis.aqi` is missing/invalid.
3. First local login subscribes `$aqi:user:<id>` exactly once.
4. Additional local clients for the same user do not resubscribe.
5. Last local client disconnect unsubscribes exactly once.
6. Same-app client replacement does not accidentally unsubscribe an online user.
7. A retained user's topics are released from Redis when offline and reacquired on same-node reconnect.
8. First online local subscriber to a normal topic subscribes `$aqi:topic:<topicId>`; last online subscriber unsubscribes.
9. Business Topic IDs beginning with `$aqi:` remain valid and are nested under `$aqi:topic:`.
10. Cross-node user send reaches the correct remote node only.
11. Cross-node topic publish reaches subscribed nodes and not unrelated nodes.
12. Redis-originated messages are delivered locally without republishing (no loop).
13. Existing `PubSub.Unsub` double-removal path is removed and covered by regression test.
14. Race tests pass for login/logout/topic lifecycle.

## Scope guard

If implementation starts requiring any of the following, stop instead of expanding scope:

- Redis keys for user/node registries
- TTL/heartbeat-based node ownership
- Streams/queues/ACK/retry
- distributed session/topic persistence
- Room/Game state movement
- distributed locks

Those are separate features, not part of `WithCluster()`.
