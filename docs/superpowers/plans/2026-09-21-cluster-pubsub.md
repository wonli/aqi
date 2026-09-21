# AQI Cluster Pub/Sub Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add opt-in Redis Pub/Sub based multi-node realtime routing behind `aqi.WithCluster()` while preserving AQI's current local User, Topic, reconnect-grace, and best-effort semantics.

**Architecture:** `WithCluster()` only enables the capability. After YAML is loaded, AQI resolves the existing `redis.aqi` store and injects its `*redis.Client` into `ws`. The `ws` package owns a small cluster runtime: node-local reference counts decide Redis `SUBSCRIBE`/`UNSUBSCRIBE`, Redis carries only encoded bytes, and Redis-originated messages enter a remote-only local delivery path that never republishes. No distributed state is introduced.

**Tech Stack:** Go 1.27, `github.com/redis/go-redis/v9` (already present), existing AQI `ws`/`store` packages, Go race detector. `miniredis` may be added as a test-only dependency for Redis transport integration tests.

**Spec:** `docs/superpowers/specs/2026-09-21-cluster-pubsub-design.md`

## Global Constraints

- Cluster is opt-in through `aqi.WithCluster()`; no option argument carries a Redis address.
- Cluster requires existing YAML config at `redis.aqi` and startup must fail clearly if the configured Redis cannot be used.
- Redis Pub/Sub is realtime best-effort transport only; no ACK, retry, replay, queue, Stream, or durable storage.
- Do not add user-to-node registries, TTL ownership, node heartbeats, distributed presence, distributed locks, Room/Game state transfer, or cross-node `User.SubTopics` persistence.
- Existing five-minute local `User`/`SubTopics` reconnect grace remains unchanged.
- Existing `PubSub.Pub` remains process-local and best-effort.
- Cluster transports bytes only; never serialize `*Client`, `*User`, `TopicMsg.Ori`, `Hub`, or arbitrary Go object graphs.
- Same-node delivery must stay local and must not depend on Redis availability.

## Review Focus

- Same-app connection replacement must not transiently mark an online user offline or unsubscribe its Redis user channel.
- Duplicate `Sub`/`Unsub` calls must not corrupt node-local topic reference counts or double-release Redis subscriptions.
- Redis-originated delivery must never call the cluster publish path and create a message loop.
- A user reconnecting to the same node during the five-minute grace period must reacquire retained topic refs; reconnecting elsewhere must not pretend to restore them.
- Concurrent login/logout/topic changes must keep ref counts non-negative and pass `go test -race ./...`.

---

### Task 1: Make Local Subscription Removal Single-Owner

**Files:**
- Modify: `ws/user.go`
- Modify: `ws/pubsub.go`
- Modify: `ws/pubsub_topic.go`
- Create/Test: `ws/pubsub_unsubscribe_test.go`

**Interfaces:**
- Consumes: existing `User.SubTopics`, `Topic.SubUsers`, `PubSub.Sub`, `PubSub.Unsub`.
- Produces: idempotent helpers that report whether a user/topic membership actually changed, so later cluster reference counting can react exactly once.

- [ ] **Step 1: Write failing regression tests**

Add tests that subscribe one user, unsubscribe once, unsubscribe again, and assert both sides (`User.SubTopics` and `Topic.SubUsers`) contain no membership after the first removal and the second removal reports no state transition. Add an `UnsubAllTopics` test covering multiple topics.

- [ ] **Step 2: Run the focused tests and verify RED**

Run:

```bash
go test ./ws -run 'TestPubSubUnsub|TestUserUnsubAllTopics' -count=1
```

Expected: FAIL because the current API does not expose a single authoritative transition and `PubSub.Unsub` plus `User.UnsubTopic` both remove the topic-side membership.

- [ ] **Step 3: Implement one authoritative removal path**

Refactor without changing public behavior:

```go
func (u *User) removeSubTopic(topicID string) (*Topic, bool)
func (a *Topic) RemoveSubUser(suid string) bool
```

`PubSub.Unsub` performs one membership removal through the helper and never calls `Topic.RemoveSubUser` a second time. `User.UnsubAllTopics` removes both sides exactly once per topic. Preserve exported compatibility methods where practical.

- [ ] **Step 4: Verify GREEN and full ws package**

```bash
go test ./ws -count=1
go test -race ./ws -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add ws/user.go ws/pubsub.go ws/pubsub_topic.go ws/pubsub_unsubscribe_test.go
git commit -m "refactor: make topic unsubscribe single-owner"
```

---

### Task 2: Add a Minimal Cluster Runtime and Reference Counting

**Files:**
- Create: `ws/cluster.go`
- Create/Test: `ws/cluster_test.go`

**Interfaces:**
- Produces:

```go
type clusterTransport interface {
    Subscribe(topic string) error
    Unsubscribe(topic string) error
    Publish(topic string, data []byte) error
    Close() error
}

func setClusterTransport(t clusterTransport)
func clearClusterTransport()
```

Internal runtime helpers:

```go
func clusterAcquire(topic string)
func clusterRelease(topic string)
func clusterPublish(topic string, data []byte) bool
```

- [ ] **Step 1: Write a fake transport and failing ref-count tests**

Tests must prove `Acquire("room:1")` twice causes one transport subscribe, first release does nothing, second release causes one unsubscribe, duplicate release never drives refs below zero, and disabled cluster is a no-op.

- [ ] **Step 2: Run focused tests and verify RED**

```bash
go test ./ws -run 'TestCluster' -count=1
```

Expected: FAIL because cluster runtime does not exist.

- [ ] **Step 3: Implement minimal runtime**

Use a mutex-protected `map[string]int`. Only edge transitions call the transport. Runtime transport errors are logged and do not break local WebSocket behavior. Do not add node IDs, registries, TTLs, retries, or durable queues.

- [ ] **Step 4: Verify GREEN including race detector**

```bash
go test ./ws -run 'TestCluster' -count=1
go test -race ./ws -run 'TestCluster' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add ws/cluster.go ws/cluster_test.go
git commit -m "feat: add local cluster subscription runtime"
```

---

### Task 3: Bind User Online/Offline Transitions to Cluster Channels

**Files:**
- Modify: `ws/user.go`
- Modify: `ws/hubc.go`
- Create/Test: `ws/cluster_user_test.go`

**Interfaces:**
- Consumes: Task 2 `clusterAcquire` / `clusterRelease`.
- Produces transition-aware login/logout behavior and internal user channel naming:

```go
func clusterUserTopic(uid string) string // e.g. "$user:B"
```

`appLogin` and `appLogout` must expose whether the local user changed between zero and non-zero `AppClients` without breaking the same-app replacement flow.

- [ ] **Step 1: Write failing lifecycle tests**

Cover:

1. first local client for B -> one `$user:B` subscribe;
2. second local client -> no extra subscribe;
3. one of two disconnects -> no unsubscribe;
4. last client disconnect -> one unsubscribe;
5. same-app replacement followed by old-client disconnect -> no false unsubscribe;
6. disabled cluster -> current login/logout behavior only.

- [ ] **Step 2: Verify RED**

```bash
go test ./ws -run 'TestClusterUser|TestSameAppCluster' -count=1
```

Expected: FAIL because user lifecycle is not cluster-aware.

- [ ] **Step 3: Implement edge-triggered user lifecycle integration**

Compute transitions while holding the existing `User` lock. Perform Redis/cluster calls after releasing the user lock. On `0 -> 1`, acquire `$user:<uid>` and reacquire all retained `SubTopics`; on `1 -> 0`, release `$user:<uid>` and release all currently retained `SubTopics`. A replaced client that is no longer present in `AppClients` must produce no offline transition when its delayed disconnect arrives.

- [ ] **Step 4: Verify GREEN and races**

```bash
go test ./ws -run 'TestClusterUser|TestSameAppCluster' -count=1
go test -race ./ws -run 'TestClusterUser|TestSameAppCluster|TestUserLogin' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add ws/user.go ws/hubc.go ws/cluster_user_test.go
git commit -m "feat: route online users through cluster channels"
```

---

### Task 4: Bind Online Topic Membership to Cluster Ref Counts

**Files:**
- Modify: `ws/pubsub.go`
- Modify: `ws/pubsub_topic.go`
- Modify: `ws/user.go` only if a small membership helper is required
- Create/Test: `ws/cluster_topic_test.go`

**Interfaces:**
- Consumes: Task 1 idempotent membership transitions; Task 2 `clusterAcquire` / `clusterRelease`; Task 3 `User.IsOnline`.
- Produces: automatic node-level Redis subscription for normal AQI topics.

- [ ] **Step 1: Write failing topic lifecycle tests**

Cover two online users on the same node subscribing the same topic (one Redis subscribe), one leaving (no Redis unsubscribe), last leaving (one unsubscribe), duplicate `Sub` (no double acquire), duplicate `Unsub` (no double release), and offline retained users not holding cluster refs.

- [ ] **Step 2: Verify RED**

```bash
go test ./ws -run 'TestClusterTopic' -count=1
```

Expected: FAIL because `PubSub.Sub/Unsub` do not update cluster refs.

- [ ] **Step 3: Implement topic edge integration**

When local user/topic membership is newly added and the user is online, acquire the topic. When membership is actually removed while online, release it. The existing five-minute `SubTopics` retention remains local; Task 3 handles releasing/reacquiring those refs when the user's online state changes.

- [ ] **Step 4: Verify GREEN and races**

```bash
go test ./ws -run 'TestClusterTopic' -count=1
go test -race ./ws -run 'TestClusterTopic|TestClusterUser' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add ws/pubsub.go ws/pubsub_topic.go ws/user.go ws/cluster_topic_test.go
git commit -m "feat: mirror online topic demand to cluster"
```

---

### Task 5: Add Cross-Node User and Topic Delivery Without Loops

**Files:**
- Modify: `ws/cluster.go`
- Modify: `ws/hubc.go`
- Modify: `ws/pubsub.go`
- Modify: `ws/pubsub_topic.go`
- Create/Test: `ws/cluster_delivery_test.go`

**Interfaces:**
- Produces explicit distributed APIs while leaving existing local APIs unchanged:

```go
func (h *Hubc) SendToUser(uid string, msg []byte) bool
func (a *PubSub) Publish(topicID string, data any) bool
```

Semantics:
- `User.SendMsg` stays local.
- `PubSub.Pub` stays local and still invokes local handlers + local users.
- `Hubc.SendToUser` sends to the local user if present and publishes bytes to `$user:<uid>` for remote nodes.
- `PubSub.Publish` performs existing local `Pub` semantics, encodes one wire payload, and publishes that payload to the cluster topic for remote WebSocket subscribers.
- Redis-originated topic payloads are delivered to local topic users only; they do **not** invoke process-local `SubFunc` handlers and do **not** republish.

- [ ] **Step 1: Write failing delivery tests**

Use fake transports/runtimes to prove local delivery happens without Redis, remote user publish uses `$user:<uid>`, normal topic publish uses the topic channel, Redis-originated user/topic data reaches local clients, and inbound delivery never increments fake transport publish count.

- [ ] **Step 2: Verify RED**

```bash
go test ./ws -run 'TestClusterDelivery|TestSendToUser|TestPublish' -count=1
```

Expected: FAIL because distributed send/publish APIs and inbound delivery do not exist.

- [ ] **Step 3: Implement minimal delivery paths**

Add a cluster inbound callback in the runtime. Distinguish `$user:` channels from normal topic channels. User inbound delivery calls only the local matching `User.SendMsg`. Topic inbound delivery constructs a `TopicMsg{TopicId: topicID, Msg: payload}` and calls only `Topic.SendToSubUser`; it must not enter `PubSub.Pub`/`Publish` again.

- [ ] **Step 4: Verify GREEN and existing PubSub semantics**

```bash
go test ./ws -count=1
go test -race ./ws -count=1
```

Expected: PASS; existing local `SubFunc` tests remain unchanged.

- [ ] **Step 5: Commit**

```bash
git add ws/cluster.go ws/hubc.go ws/pubsub.go ws/pubsub_topic.go ws/cluster_delivery_test.go
git commit -m "feat: add cross-node user and topic delivery"
```

---

### Task 6: Implement Redis Pub/Sub Transport

**Files:**
- Create: `ws/cluster_redis.go`
- Create/Test: `ws/cluster_redis_test.go`
- Modify: `go.mod` / `go.sum` only if `miniredis` is required for tests

**Interfaces:**
- Consumes: Task 2 `clusterTransport` and Task 5 inbound callback.
- Produces:

```go
func newRedisCluster(client *redis.Client, onMessage func(topic string, data []byte)) (clusterTransport, error)
```

- [ ] **Step 1: Write failing Redis transport tests**

Using `miniredis` (test-only) if needed, prove subscribe/publish/inbound callback, unsubscribe, multiple channels over one Pub/Sub object, and clean close. Add a regression test that two logical local clients do not create two Redis subscribers when the runtime has only one node-level ref.

- [ ] **Step 2: Verify RED**

```bash
go test ./ws -run 'TestRedisCluster' -count=1
```

Expected: FAIL because Redis transport does not exist.

- [ ] **Step 3: Implement one Redis Pub/Sub connection per AQI node**

Use the existing `*redis.Client` for publishing and one lazily-created `redis.PubSub` receiver for dynamic channel subscriptions. Keep receive-loop ownership inside `redisCluster`; call the supplied inbound callback with channel and copied payload bytes. Rely on go-redis Pub/Sub reconnect behavior; do not implement an AQI heartbeat/registry/retry layer.

- [ ] **Step 4: Verify GREEN**

```bash
go test ./ws -run 'TestRedisCluster' -count=1
go test -race ./ws -run 'TestRedisCluster|TestCluster' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add ws/cluster_redis.go ws/cluster_redis_test.go go.mod go.sum
git commit -m "feat: add redis cluster transport"
```

---

### Task 7: Wire `WithCluster()` to Existing `redis.aqi` Configuration

**Files:**
- Modify: `options.go`
- Modify: `app.go`
- Create/Test: `cluster_option_test.go` or `app_cluster_test.go`

**Interfaces:**
- Produces public startup option:

```go
func WithCluster() Option
```

and `AppConfig` state such as:

```go
Cluster bool
```

Startup sequence after YAML load:

```go
client := store.Redis("redis.aqi").Use()
// validate redis.aqi exists, Ping with a bounded startup context,
// create ws Redis cluster transport, then continue ws.InitManager().
```

- [ ] **Step 1: Write failing configuration/bootstrap tests**

Test the bootstrap helper directly rather than testing `os.Exit`: disabled cluster requires no Redis; enabled cluster with missing `redis.aqi` returns a clear error; enabled cluster with a test Redis succeeds and installs the transport; unusable Redis returns a clear startup error.

- [ ] **Step 2: Verify RED**

```bash
go test . -run 'TestClusterOption|TestClusterBootstrap' -count=1
```

Expected: FAIL because option/bootstrap do not exist.

- [ ] **Step 3: Implement startup wiring**

`WithCluster()` only flips the config flag. Do not read Viper inside the Option because options run before config loading. After config is loaded and logging is initialized, validate `redis.aqi`, obtain the existing named Redis store, Ping with a short bounded context, then call the ws cluster initializer before normal WebSocket use.

- [ ] **Step 4: Verify GREEN**

```bash
go test . -run 'TestClusterOption|TestClusterBootstrap' -count=1
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add options.go app.go cluster_option_test.go app_cluster_test.go
git commit -m "feat: enable cluster mode from redis.aqi"
```

---

### Task 8: End-to-End Regression and Race Verification

**Files:**
- Modify only tests/docs if verification reveals a missing regression case.

**Interfaces:**
- Consumes all previous tasks.
- Produces a branch verified against the design boundaries.

- [ ] **Step 1: Run formatting and vet**

```bash
gofmt -w ws/*.go *.go
go vet ./...
```

Expected: no vet errors.

- [ ] **Step 2: Run complete tests**

```bash
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 3: Run complete race suite**

```bash
go test -race ./... -count=1
```

Expected: PASS.

- [ ] **Step 4: Inspect the branch for forbidden scope growth**

Check that the diff contains none of: Redis user/node registry keys, TTL ownership, node heartbeat, Streams/queues, ACK/retry, Room/Game state synchronization, distributed locks, or persisted `SubTopics`.

- [ ] **Step 5: Final commit only if verification required test/doc cleanup**

```bash
git add -A
git commit -m "test: harden cluster lifecycle coverage"
```
