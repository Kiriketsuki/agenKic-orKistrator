package state

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	fieldState           = "state"
	fieldLastHeartbeat   = "last_heartbeat"
	fieldCurrentTask     = "current_task_id"
	fieldCurrentTaskPrio = "current_task_priority"
	fieldRegisteredAt    = "registered_at"

	streamKey = "events"
	queueKey  = "task_queue"

	agentSetKey = "agents"
)

// RedisStore implements StateStore using Redis.
//
// Key layout (all keys are prefixed with keyPrefix):
//
//	{prefix}agent:{id}   → Hash  (state, last_heartbeat, current_task_id, registered_at)
//	{prefix}agents       → Set   (agent IDs currently registered)
//	{prefix}events       → Stream (event log)
//	{prefix}task_queue   → Sorted Set (score=priority, member=task_id)
type RedisStore struct {
	client    *redis.Client
	keyPrefix string
}

// Option is a functional option for NewRedisStore.
type Option func(*RedisStore)

// WithKeyPrefix sets a namespace prefix for all Redis keys. Useful for test
// isolation (e.g. `test:{uuid}:`).
func WithKeyPrefix(prefix string) Option {
	return func(r *RedisStore) {
		r.keyPrefix = prefix
	}
}

// NewRedisStore dials Redis at the given URL and returns a ready RedisStore.
// Returns an error if the connection cannot be established.
func NewRedisStore(redisURL string, opts ...Option) (*RedisStore, error) {
	parsed, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis URL: %w", err)
	}

	client := redis.NewClient(parsed)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	store := &RedisStore{client: client}
	for _, o := range opts {
		o(store)
	}
	return store, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func (r *RedisStore) key(parts ...string) string {
	k := r.keyPrefix
	for _, p := range parts {
		k += p
	}
	return k
}

func (r *RedisStore) agentKey(agentID string) string {
	return r.key("agent:", agentID)
}

// ── Agent state ───────────────────────────────────────────────────────────────

func (r *RedisStore) SetAgentState(ctx context.Context, agentID string, state string) error {
	pipe := r.client.TxPipeline()
	pipe.HSet(ctx, r.agentKey(agentID), fieldState, state)
	pipe.SAdd(ctx, r.key(agentSetKey), agentID)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("SetAgentState %s: %w", agentID, err)
	}
	return nil
}

func (r *RedisStore) GetAgentState(ctx context.Context, agentID string) (string, error) {
	val, err := r.client.HGet(ctx, r.agentKey(agentID), fieldState).Result()
	if err == redis.Nil {
		return "", ErrAgentNotFound
	}
	if err != nil {
		return "", fmt.Errorf("GetAgentState %s: %w", agentID, err)
	}
	return val, nil
}

// casScript atomically compares the current state field of an agent hash and
// sets it to the new value only if it matches the expected value.
//
// KEYS[1] = agent hash key
// ARGV[1] = expected state
// ARGV[2] = next state
//
// Returns:
//
//	1  → swap succeeded
//	0  → current state did not match expected (conflict)
//	-1 → key does not exist or has no state field (agent not found)
var casScript = redis.NewScript(`
local current = redis.call('HGET', KEYS[1], 'state')
if current == false then
    return -1
end
if current ~= ARGV[1] then
    return {0, current}
end
redis.call('HSET', KEYS[1], 'state', ARGV[2])
return 1
`)

// clearTaskScript atomically checks key existence before clearing task fields.
// KEYS[1] = agent hash key
//
// Returns:
//
//	1  → cleared successfully
//	-1 → key does not exist (agent not found)
//
// Field names must match fieldCurrentTask / fieldCurrentTaskPrio constants.
var clearTaskScript = redis.NewScript(`
local exists = redis.call('EXISTS', KEYS[1])
if exists == 0 then
    return -1
end
redis.call('HSET', KEYS[1], 'current_task_id', '', 'current_task_priority', '0')
return 1
`)

func (r *RedisStore) CompareAndSetAgentState(ctx context.Context, agentID string, expected, next string) error {
	raw, err := casScript.Run(ctx, r.client, []string{r.agentKey(agentID)}, expected, next).Result()
	if err != nil {
		return fmt.Errorf("CompareAndSetAgentState %s: %w", agentID, err)
	}
	switch v := raw.(type) {
	case int64:
		switch v {
		case 1:
			return nil
		case -1:
			return ErrAgentNotFound
		default:
			return fmt.Errorf("CompareAndSetAgentState %s: unexpected Lua result %d", agentID, v)
		}
	case []interface{}:
		// Conflict: Lua returned {0, current_state}.
		actual := "<unknown>"
		if len(v) >= 2 {
			if s, ok := v[1].(string); ok {
				actual = s
			}
		}
		return &StateConflictError{Expected: expected, Actual: actual}
	default:
		return fmt.Errorf("CompareAndSetAgentState %s: unexpected Lua result type %T", agentID, raw)
	}
}

// ── Agent full record ─────────────────────────────────────────────────────────

func (r *RedisStore) SetAgentFields(ctx context.Context, agentID string, fields AgentFields) error {
	pipe := r.client.TxPipeline()
	pipe.HSet(ctx, r.agentKey(agentID),
		fieldState, fields.State,
		fieldLastHeartbeat, strconv.FormatInt(fields.LastHeartbeat, 10),
		fieldCurrentTask, fields.CurrentTaskID,
		fieldCurrentTaskPrio, strconv.FormatFloat(fields.CurrentTaskPriority, 'f', -1, 64),
		fieldRegisteredAt, strconv.FormatInt(fields.RegisteredAt, 10),
	)
	pipe.SAdd(ctx, r.key(agentSetKey), agentID)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("SetAgentFields %s: %w", agentID, err)
	}
	return nil
}

func (r *RedisStore) GetAgentFields(ctx context.Context, agentID string) (AgentFields, error) {
	vals, err := r.client.HGetAll(ctx, r.agentKey(agentID)).Result()
	if err != nil {
		return AgentFields{}, fmt.Errorf("GetAgentFields %s: %w", agentID, err)
	}
	if len(vals) == 0 {
		return AgentFields{}, ErrAgentNotFound
	}

	var lhb int64
	if v := vals[fieldLastHeartbeat]; v != "" {
		lhb, err = strconv.ParseInt(v, 10, 64)
		if err != nil {
			return AgentFields{}, fmt.Errorf("GetAgentFields %s: parse %s: %w", agentID, fieldLastHeartbeat, err)
		}
	}
	var ra int64
	if v := vals[fieldRegisteredAt]; v != "" {
		ra, err = strconv.ParseInt(v, 10, 64)
		if err != nil {
			return AgentFields{}, fmt.Errorf("GetAgentFields %s: parse %s: %w", agentID, fieldRegisteredAt, err)
		}
	}
	var ctp float64
	if v := vals[fieldCurrentTaskPrio]; v != "" {
		ctp, err = strconv.ParseFloat(v, 64)
		if err != nil {
			return AgentFields{}, fmt.Errorf("GetAgentFields %s: parse %s: %w", agentID, fieldCurrentTaskPrio, err)
		}
	}

	return AgentFields{
		State:               vals[fieldState],
		LastHeartbeat:       lhb,
		CurrentTaskID:       vals[fieldCurrentTask],
		CurrentTaskPriority: ctp,
		RegisteredAt:        ra,
	}, nil
}

func (r *RedisStore) ClearCurrentTask(ctx context.Context, agentID string) error {
	result, err := clearTaskScript.Run(ctx, r.client, []string{r.agentKey(agentID)}).Int64()
	if err != nil {
		return fmt.Errorf("ClearCurrentTask %s: %w", agentID, err)
	}
	if result == -1 {
		return ErrAgentNotFound
	}
	return nil
}

func (r *RedisStore) DeleteAgent(ctx context.Context, agentID string) error {
	pipe := r.client.TxPipeline()
	pipe.Del(ctx, r.agentKey(agentID))
	pipe.SRem(ctx, r.key(agentSetKey), agentID)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("DeleteAgent %s: %w", agentID, err)
	}
	return nil
}

func (r *RedisStore) ListAgents(ctx context.Context) ([]string, error) {
	members, err := r.client.SMembers(ctx, r.key(agentSetKey)).Result()
	if err != nil {
		return nil, fmt.Errorf("ListAgents: %w", err)
	}
	return members, nil
}

func (r *RedisStore) GetAllAgentStates(ctx context.Context) (map[string]string, error) {
	members, err := r.client.SMembers(ctx, r.key(agentSetKey)).Result()
	if err != nil {
		return nil, fmt.Errorf("GetAllAgentStates: list members: %w", err)
	}
	if len(members) == 0 {
		return map[string]string{}, nil
	}

	pipe := r.client.Pipeline()
	cmds := make([]*redis.StringCmd, len(members))
	for i, id := range members {
		cmds[i] = pipe.HGet(ctx, r.agentKey(id), fieldState)
	}
	if _, err = pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, fmt.Errorf("GetAllAgentStates: pipeline: %w", err)
	}

	states := make(map[string]string, len(members))
	for i, id := range members {
		val, cmdErr := cmds[i].Result()
		if cmdErr == redis.Nil {
			continue
		}
		if cmdErr != nil {
			return nil, fmt.Errorf("GetAllAgentStates: get state for %s: %w", id, cmdErr)
		}
		states[id] = val
	}
	return states, nil
}

// ── Event stream ──────────────────────────────────────────────────────────────

func (r *RedisStore) PublishEvent(ctx context.Context, event Event) error {
	ts := event.Timestamp
	if ts == 0 {
		ts = time.Now().UnixMilli()
	}

	args := &redis.XAddArgs{
		Stream: r.key(streamKey),
		ID:     "*",
		Values: map[string]any{
			"type":      event.Type,
			"agent_id":  event.AgentID,
			"task_id":   event.TaskID,
			"timestamp": strconv.FormatInt(ts, 10),
			"payload":   event.Payload,
		},
	}
	if err := r.client.XAdd(ctx, args).Err(); err != nil {
		return fmt.Errorf("PublishEvent: %w", err)
	}
	return nil
}

func parseXMessages(msgs []redis.XMessage) []StreamEvent {
	events := make([]StreamEvent, 0, len(msgs))
	for _, msg := range msgs {
		var ts int64
		if v, ok := msg.Values["timestamp"].(string); ok && v != "" {
			ts, _ = strconv.ParseInt(v, 10, 64)
		}
		events = append(events, StreamEvent{
			ID: msg.ID,
			Event: Event{
				Type:      fmt.Sprintf("%v", msg.Values["type"]),
				AgentID:   fmt.Sprintf("%v", msg.Values["agent_id"]),
				TaskID:    fmt.Sprintf("%v", msg.Values["task_id"]),
				Timestamp: ts,
				Payload:   fmt.Sprintf("%v", msg.Values["payload"]),
			},
		})
	}
	return events
}

func (r *RedisStore) ReadEvents(ctx context.Context, lastID string, count int64) ([]StreamEvent, error) {
	results, err := r.client.XRead(ctx, &redis.XReadArgs{
		Streams: []string{r.key(streamKey), lastID},
		Count:   count,
		Block:   -1, // negative = non-blocking (omits BLOCK arg); 0 would block forever
	}).Result()
	if err == redis.Nil {
		return []StreamEvent{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ReadEvents: %w", err)
	}
	if len(results) == 0 {
		return []StreamEvent{}, nil
	}
	return parseXMessages(results[0].Messages), nil
}

func (r *RedisStore) CreateConsumerGroup(ctx context.Context, group string, startID string) error {
	err := r.client.XGroupCreateMkStream(ctx, r.key(streamKey), group, startID).Err()
	if err != nil && strings.Contains(err.Error(), "BUSYGROUP") {
		return nil
	}
	if err != nil {
		return fmt.Errorf("CreateConsumerGroup: %w", err)
	}
	return nil
}

func (r *RedisStore) SubscribeEvents(ctx context.Context, group, consumer string, count int64, block time.Duration) ([]StreamEvent, error) {
	// go-redis: Block 0 = BLOCK 0 (block forever). Use -1 to omit the
	// BLOCK arg entirely, making the call non-blocking when block <= 0.
	if block <= 0 {
		block = -1
	}
	results, err := r.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{r.key(streamKey), ">"},
		Count:    count,
		Block:    block,
	}).Result()
	if err == redis.Nil {
		return []StreamEvent{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("SubscribeEvents: %w", err)
	}
	if len(results) == 0 {
		return []StreamEvent{}, nil
	}
	return parseXMessages(results[0].Messages), nil
}

func (r *RedisStore) AckEvent(ctx context.Context, group string, ids ...string) error {
	if err := r.client.XAck(ctx, r.key(streamKey), group, ids...).Err(); err != nil {
		return fmt.Errorf("AckEvent: %w", err)
	}
	return nil
}

// ── Task queue ────────────────────────────────────────────────────────────────

func (r *RedisStore) EnqueueTask(ctx context.Context, taskID string, priority float64) error {
	err := r.client.ZAdd(ctx, r.key(queueKey), redis.Z{
		Score:  priority,
		Member: taskID,
	}).Err()
	if err != nil {
		return fmt.Errorf("EnqueueTask %s: %w", taskID, err)
	}
	return nil
}

func (r *RedisStore) DequeueTask(ctx context.Context) (string, float64, error) {
	// ZPOPMIN atomically removes and returns the member with the lowest score.
	results, err := r.client.ZPopMin(ctx, r.key(queueKey), 1).Result()
	if err != nil {
		return "", 0, fmt.Errorf("DequeueTask: %w", err)
	}
	if len(results) == 0 {
		return "", 0, ErrQueueEmpty
	}
	member, ok := results[0].Member.(string)
	if !ok {
		return "", 0, fmt.Errorf("DequeueTask: unexpected member type %T", results[0].Member)
	}
	return member, results[0].Score, nil
}

func (r *RedisStore) QueueLength(ctx context.Context) (int64, error) {
	n, err := r.client.ZCard(ctx, r.key(queueKey)).Result()
	if err != nil {
		return 0, fmt.Errorf("QueueLength: %w", err)
	}
	return n, nil
}

// ── Lifecycle ─────────────────────────────────────────────────────────────────

func (r *RedisStore) Ping(ctx context.Context) error {
	if err := r.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("Ping: %w", err)
	}
	return nil
}

func (r *RedisStore) Close() error {
	return r.client.Close()
}
