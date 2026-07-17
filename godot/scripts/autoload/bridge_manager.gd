extends Node
## BridgeManager — HTTP/SSE connection to the orchestrator.
## Autoloaded singleton. Handles initial sync, live SSE updates, and write commands.

signal agent_registered(agent_data: BridgeData.AgentData)
signal agent_state_changed(agent_id: String, old_state: String, new_state: String, task_id: String)
signal agent_deregistered(agent_id: String)
signal agent_output(chunk: BridgeData.AgentOutputChunk)
## Emitted once per fetch_agent_output_history() call, with the parsed backfill
## as an Array[BridgeData.AgentOutputChunk] (empty on any failure/parse miss).
signal agent_output_history(agent_id: String, chunks: Array)
signal connection_status_changed(status: String)
## Emitted when a queued write command (submit_task, submit_dag, ...) fails —
## either a transport failure or a non-2xx HTTP response. `reason` is the
## server's `error` field when the body could be parsed, or a generic
## fallback message otherwise (see _extract_error_reason).
signal command_failed(path: String, code: int, reason: String)
## Emitted when a queued write command succeeds (2xx). `response_body` is the
## raw JSON response body as a string; consumers (e.g. the quest board) parse
## the fields they care about (task_id, dag_execution_id, ...) themselves.
signal command_succeeded(path: String, code: int, response_body: String)
signal floor_created(floor_data: BridgeData.FloorData)
signal floor_removed(floor_name: String)

enum ConnectionState { DISCONNECTED, CONNECTING, CONNECTED, RECONNECTING }

@export var base_url: String = "http://localhost:8081"

var _connection_state: ConnectionState = ConnectionState.DISCONNECTED
var _agent_states: Dictionary = {}
var _agents: Dictionary = {}
var _floors: Array[BridgeData.FloorData] = []
var _sse_client: HTTPClient
var _sse_buffer: String = ""
var _sse_request_sent: bool = false
var _last_cursor: String = ""
var _last_data_time: float = 0.0
var _backoff_seconds: float = 1.0
var _backoff_timer: float = 0.0
var _command_queue: Array[Dictionary] = []
var _command_in_flight: bool = false
var _inflight_path: String = ""

var _sync_agents_request: HTTPRequest
var _sync_floors_request: HTTPRequest
var _command_request: HTTPRequest
var _output_history_request: HTTPRequest
var _output_history_agent_id: String = ""

var _agents_synced: bool = false
var _floors_synced: bool = false
var _initial_sync_failed: bool = false


func _ready() -> void:
	_sync_agents_request = HTTPRequest.new()
	add_child(_sync_agents_request)
	_sync_agents_request.timeout = 10
	_sync_agents_request.request_completed.connect(_on_agents_synced)

	_sync_floors_request = HTTPRequest.new()
	add_child(_sync_floors_request)
	_sync_floors_request.timeout = 10
	_sync_floors_request.request_completed.connect(_on_floors_synced)

	_command_request = HTTPRequest.new()
	add_child(_command_request)
	_command_request.timeout = 10
	_command_request.request_completed.connect(_on_command_completed)

	_output_history_request = HTTPRequest.new()
	add_child(_output_history_request)
	_output_history_request.timeout = 10
	_output_history_request.request_completed.connect(_on_output_history_completed)

	_set_connection_state(ConnectionState.CONNECTING)
	_start_initial_sync()


func _process(delta: float) -> void:
	match _connection_state:
		ConnectionState.CONNECTED:
			_poll_sse(delta)
			_check_keepalive_timeout()
		ConnectionState.DISCONNECTED, ConnectionState.RECONNECTING:
			_update_backoff_timer(delta)


# --- Connection State Machine ---

func _set_connection_state(new_state: ConnectionState) -> void:
	if _connection_state == new_state:
		return
	_connection_state = new_state
	var status_map: Dictionary = {
		ConnectionState.DISCONNECTED: "disconnected",
		ConnectionState.CONNECTING: "connecting",
		ConnectionState.CONNECTED: "connected",
		ConnectionState.RECONNECTING: "reconnecting",
	}
	connection_status_changed.emit(status_map[new_state])


# --- Initial Sync ---

func _start_initial_sync() -> void:
	_agents_synced = false
	_floors_synced = false
	_initial_sync_failed = false
	var err_agents := _sync_agents_request.request(base_url + "/api/agents")
	var err_floors := _sync_floors_request.request(base_url + "/api/floors")
	if err_agents != OK or err_floors != OK:
		_on_initial_sync_failed()


# NOTE: On reconnect, agent_registered is emitted for ALL agents via REST sync,
# then SSE replay may re-emit for agents registered during the disconnect window.
# This is at-least-once delivery by design. Consumers must be idempotent.
func _on_agents_synced(result: int, code: int, _headers: PackedStringArray, body: PackedByteArray) -> void:
	if result != HTTPRequest.RESULT_SUCCESS or code != 200:
		_on_initial_sync_failed()
		return
	var parsed: Variant = JSON.parse_string(body.get_string_from_utf8())
	if not parsed is Dictionary:
		_on_initial_sync_failed()
		return
	var agents_array: Variant = (parsed as Dictionary).get("agents", null)
	if not agents_array is Array:
		_on_initial_sync_failed()
		return
	_agents.clear()
	for item: Variant in (agents_array as Array):
		if not item is Dictionary:
			continue
		var agent: BridgeData.AgentData = BridgeData.AgentData.from_dict(item as Dictionary)
		_agent_states[agent.id] = agent.state
		_agents[agent.id] = agent
		agent_registered.emit(agent)
	_agents_synced = true
	_check_initial_sync_complete()


func _on_floors_synced(result: int, code: int, _headers: PackedStringArray, body: PackedByteArray) -> void:
	if result != HTTPRequest.RESULT_SUCCESS or code != 200:
		_on_initial_sync_failed()
		return
	var parsed: Variant = JSON.parse_string(body.get_string_from_utf8())
	if not parsed is Dictionary:
		_on_initial_sync_failed()
		return
	var floors_array: Variant = (parsed as Dictionary).get("floors", null)
	if not floors_array is Array:
		_on_initial_sync_failed()
		return
	_floors.clear()
	for item: Variant in (floors_array as Array):
		if not item is Dictionary:
			continue
		_floors.append(BridgeData.FloorData.from_dict(item as Dictionary))
	_floors_synced = true
	_check_initial_sync_complete()


func _check_initial_sync_complete() -> void:
	if _initial_sync_failed:
		return
	if not (_agents_synced and _floors_synced):
		return
	_open_sse_stream()
	_set_connection_state(ConnectionState.CONNECTED)
	_last_data_time = Time.get_unix_time_from_system()
	_backoff_seconds = 1.0
	if not _command_queue.is_empty() and not _command_in_flight:
		_process_next_command()


func _on_initial_sync_failed() -> void:
	if _initial_sync_failed:
		return
	_initial_sync_failed = true
	if _connection_state == ConnectionState.RECONNECTING:
		_backoff_seconds = minf(_backoff_seconds * 2.0, 30.0)
	_set_connection_state(ConnectionState.DISCONNECTED)
	_backoff_timer = _backoff_seconds


# --- SSE Stream ---

func _open_sse_stream() -> void:
	_sse_client = HTTPClient.new()
	_sse_buffer = ""
	_sse_request_sent = false
	var url: String = base_url
	if url.begins_with("http://"):
		url = url.substr(7)
	elif url.begins_with("https://"):
		url = url.substr(8)
	var host: String = url.split("/")[0]
	var port: int = 443 if base_url.begins_with("https://") else 80
	if ":" in host:
		var parts: PackedStringArray = host.split(":")
		host = parts[0]
		port = parts[1].to_int()
	_sse_client.connect_to_host(host, port)


func _poll_sse(_delta: float) -> void:
	if _sse_client == null:
		return
	_sse_client.poll()
	var status: HTTPClient.Status = _sse_client.get_status()
	match status:
		HTTPClient.STATUS_CANT_CONNECT, HTTPClient.STATUS_CANT_RESOLVE, \
		HTTPClient.STATUS_CONNECTION_ERROR, HTTPClient.STATUS_TLS_HANDSHAKE_ERROR:
			_on_sse_disconnected()
		HTTPClient.STATUS_CONNECTED:
			if not _sse_request_sent:
				var cursor_param: String = ""
				if _last_cursor != "":
					cursor_param = "?since=" + _last_cursor
				_sse_client.request(
					HTTPClient.METHOD_GET,
					"/events/stream" + cursor_param,
					["Accept: text/event-stream"]
				)
				_sse_request_sent = true
		HTTPClient.STATUS_BODY:
			var chunk: PackedByteArray = _sse_client.read_response_body_chunk()
			if chunk.size() == 0:
				return
			_last_data_time = Time.get_unix_time_from_system()
			_sse_buffer += chunk.get_string_from_utf8()
			if _sse_buffer.length() > 1_048_576:
				_sse_buffer = ""
				_on_sse_disconnected()
				return
			var events: PackedStringArray = _sse_buffer.split("\n\n")
			_sse_buffer = events[events.size() - 1]
			for i: int in range(events.size() - 1):
				var raw: String = events[i].strip_edges()
				if raw == "":
					continue
				var event: Dictionary = _parse_sse_event(raw)
				if event.is_empty():
					continue
				_dispatch_sse_event(event.get("type", ""), event.get("data", {}))


func _parse_sse_event(raw: String) -> Dictionary:
	var event_type: String = ""
	var data_str: String = ""
	for line: String in raw.split("\n"):
		if line.begins_with(":"):
			_last_data_time = Time.get_unix_time_from_system()
			continue
		if line.begins_with("event:"):
			event_type = line.substr(6).strip_edges()
		elif line.begins_with("data:"):
			if data_str != "":
				data_str += "\n"
			data_str += line.substr(5).strip_edges()
	if event_type == "" or data_str == "":
		return {}
	var parsed: Variant = JSON.parse_string(data_str)
	if not parsed is Dictionary:
		return {}
	return {"type": event_type, "data": parsed as Dictionary}


# --- SSE Event Dispatch ---

func _dispatch_sse_event(event_type: String, data: Dictionary) -> void:
	match event_type:
		"agent.registered":
			var agent: BridgeData.AgentData = BridgeData.AgentData.from_dict(data)
			_agent_states[agent.id] = agent.state
			_agents[agent.id] = agent
			agent_registered.emit(agent)
		"agent.state_changed":
			var agent_id: String = data.get("agent_id", "")
			var new_state: String = data.get("state", "")
			var task_id: String = data.get("task_id", "")
			var old_state: String = _agent_states.get(agent_id, "")
			if _agents.has(agent_id):
				_agents[agent_id].state = new_state
			agent_state_changed.emit(agent_id, old_state, new_state, task_id)
			_agent_states[agent_id] = new_state
		"agent.deregistered":
			var agent_id: String = data.get("agent_id", "")
			if agent_id != "":
				_agent_states.erase(agent_id)
				_agents.erase(agent_id)
				agent_deregistered.emit(agent_id)
		"agent.output":
			var chunk: BridgeData.AgentOutputChunk = BridgeData.AgentOutputChunk.from_dict(data)
			agent_output.emit(chunk)
		"floor.created":
			var floor_data: BridgeData.FloorData = BridgeData.FloorData.from_dict(data)
			_floors.append(floor_data)
			floor_created.emit(floor_data)
		"floor.removed":
			var floor_name: String = data.get("name", "")
			_floors = _floors.filter(func(f: BridgeData.FloorData) -> bool: return f.name != floor_name)
			floor_removed.emit(floor_name)
	if data.has("cursor"):
		_last_cursor = str(data["cursor"])


# --- Reconnection ---

func _check_keepalive_timeout() -> void:
	if Time.get_unix_time_from_system() - _last_data_time > 20.0:
		_on_sse_disconnected()


func _on_sse_disconnected() -> void:
	if _sse_client != null:
		_sse_client.close()
		_sse_client = null
	_set_connection_state(ConnectionState.DISCONNECTED)
	_backoff_timer = _backoff_seconds


func _update_backoff_timer(delta: float) -> void:
	if _backoff_timer <= 0.0:
		return
	_backoff_timer -= delta
	if _backoff_timer <= 0.0:
		_start_reconnect()


func _start_reconnect() -> void:
	_set_connection_state(ConnectionState.RECONNECTING)
	_start_initial_sync()


# --- Write Commands ---

## Submits a single quest-board task (#118). `description` is the only
## required field — `task_id` may be left blank to let the server generate
## one. `priority` follows DequeueTask's ZPOPMIN semantics: LOWER numeric
## values are dequeued FIRST, so callers mapping a High/Normal/Low picker
## must map High -> a low number (e.g. 1), not a high one.
func submit_task(description: String, priority: float, floor: String = "", project: String = "", task_id: String = "") -> void:
	_enqueue_command("POST", "/api/tasks", {
		"task_id": task_id,
		"priority": priority,
		"description": description,
		"floor": floor,
		"project": project,
	})


func submit_dag(nodes: Array, edges: Array) -> void:
	_enqueue_command("POST", "/api/dags", {"nodes": nodes, "edges": edges})


func send_input(agent_id: String, keys: String) -> void:
	_enqueue_command("POST", "/api/agents/" + agent_id + "/input", {"keys": keys})


## Requeues agent_id's current task with a tier/provider hint (T14 / #119
## "Reassign task"). `target` is passed straight through as the JSON body —
## callers set whichever of {"tier": ..., "provider": ...} applies. See
## httpbridge.ReassignAgentRequest doc comment: this is a persisted hint on
## the requeued task, not live migration — the assign loop does not honor it
## yet. Result surfaces via command_succeeded/command_failed (path
## "/api/agents/{id}/reassign").
func reassign_agent(agent_id: String, target: Dictionary) -> void:
	_enqueue_command("POST", "/api/agents/" + agent_id + "/reassign", target)


## Cancels agent_id's current task (T14 / #119 "Cancel task"). Result
## surfaces via command_succeeded/command_failed (path
## "/api/agents/{id}/cancel").
func cancel_agent_task(agent_id: String) -> void:
	_enqueue_command("POST", "/api/agents/" + agent_id + "/cancel", {})


func get_agent_output(agent_id: String, lines: int = 50) -> void:
	_enqueue_command("GET", "/api/agents/" + agent_id + "/output?lines=" + str(lines), {})


## Dedicated GET for scroll-panel backfill — unlike get_agent_output() above
## (which is a fire-and-forget write-command-queue entry whose response body
## is discarded), this reads and parses the response body and emits
## agent_output_history with the result.
##
## Only one backfill can be in flight on the shared _output_history_request
## node at a time. Any previous in-flight request is cancelled first —
## cancel_request() does NOT fire request_completed in Godot 4, so the
## stale request's response can never land and be mislabeled with the new
## agent_id. _output_history_agent_id is only set once we know the new
## request actually started, so a failed/aborted request() call never
## stomps the id of a request that is still genuinely in flight.
func fetch_agent_output_history(agent_id: String, lines: int = 200) -> void:
	_output_history_request.cancel_request()
	var url: String = base_url + "/api/agents/" + agent_id + "/output?lines=" + str(lines)
	var err: int = _output_history_request.request(url)
	if err != OK:
		agent_output_history.emit(agent_id, [])
		return
	_output_history_agent_id = agent_id


func _on_output_history_completed(result: int, code: int, _headers: PackedStringArray, body: PackedByteArray) -> void:
	var agent_id: String = _output_history_agent_id
	if result != HTTPRequest.RESULT_SUCCESS or code < 200 or code >= 300:
		agent_output_history.emit(agent_id, [])
		return
	var parsed: Variant = JSON.parse_string(body.get_string_from_utf8())
	agent_output_history.emit(agent_id, _parse_output_history(parsed, agent_id))


## Defensive parse: the /output response shape is undocumented, so this
## tolerates {output:[...]}, {lines:[...]}, {chunks:[...]}, or a bare top-level
## array — and each item may be either a chunk dict or a plain output string.
func _parse_output_history(parsed: Variant, agent_id: String) -> Array:
	var chunks: Array = []
	var raw_list: Variant = null
	if parsed is Array:
		raw_list = parsed
	elif parsed is Dictionary:
		var dict: Dictionary = parsed as Dictionary
		if dict.get("output", null) is Array:
			raw_list = dict.get("output")
		elif dict.get("lines", null) is Array:
			raw_list = dict.get("lines")
		elif dict.get("chunks", null) is Array:
			raw_list = dict.get("chunks")
	if not raw_list is Array:
		return chunks
	for item: Variant in (raw_list as Array):
		var chunk: BridgeData.AgentOutputChunk = _coerce_output_item(item, agent_id)
		if chunk != null:
			chunks.append(chunk)
	return chunks


func _coerce_output_item(item: Variant, agent_id: String) -> BridgeData.AgentOutputChunk:
	if item is Dictionary:
		var d: Dictionary = item as Dictionary
		if d.get("agent_id", "") == "":
			d = d.duplicate()
			d["agent_id"] = agent_id
		return BridgeData.AgentOutputChunk.from_dict(d)
	if item is String:
		var chunk: BridgeData.AgentOutputChunk = BridgeData.AgentOutputChunk.new()
		chunk.agent_id = agent_id
		chunk.payload = item as String
		return chunk
	return null


func get_agent(agent_id: String) -> BridgeData.AgentData:
	return _agents.get(agent_id, null)


func _enqueue_command(method: String, path: String, body: Dictionary) -> void:
	_command_queue.append({"method": method, "path": path, "body": body})
	if _connection_state != ConnectionState.CONNECTED:
		return
	if not _command_in_flight:
		_process_next_command()


func _process_next_command() -> void:
	if _command_queue.is_empty():
		return
	if _connection_state != ConnectionState.CONNECTED:
		return
	var cmd: Dictionary = _command_queue.pop_front()
	_inflight_path = cmd["path"]
	_command_in_flight = true
	var http_method: HTTPClient.Method = HTTPClient.METHOD_GET
	if cmd["method"] == "POST":
		http_method = HTTPClient.METHOD_POST
	var body_str: String = ""
	if http_method == HTTPClient.METHOD_POST:
		body_str = JSON.stringify(cmd["body"])
	_command_request.request(
		base_url + cmd["path"],
		["Content-Type: application/json"],
		http_method,
		body_str
	)


func _on_command_completed(result: int, code: int, _headers: PackedStringArray, body: PackedByteArray) -> void:
	var body_str: String = body.get_string_from_utf8()
	if result != HTTPRequest.RESULT_SUCCESS or code < 200 or code >= 300:
		push_warning("BridgeManager: command failed (result=%d, code=%d, path=%s)" % [result, code, _inflight_path])
		command_failed.emit(_inflight_path, code, _extract_error_reason(body_str, result, code))
	else:
		command_succeeded.emit(_inflight_path, code, body_str)
	_command_in_flight = false
	_process_next_command()


## Best-effort extraction of a human-readable failure reason from a JSON
## error body ({"error": "...", "code": "..."}), falling back to a generic
## message when the body is empty/unparseable or the request never reached
## the server (transport-level failure).
func _extract_error_reason(body_str: String, result: int, code: int) -> String:
	if result != HTTPRequest.RESULT_SUCCESS:
		return "connection failed (result=%d)" % result
	if body_str != "":
		var parsed: Variant = JSON.parse_string(body_str)
		if parsed is Dictionary and (parsed as Dictionary).get("error", "") != "":
			return str((parsed as Dictionary)["error"])
	return "request failed (code=%d)" % code


func get_registered_agents() -> Array[BridgeData.AgentData]:
	var agents: Array[BridgeData.AgentData] = []
	for agent_id: String in _agents:
		agents.append(_agents[agent_id])
	return agents
