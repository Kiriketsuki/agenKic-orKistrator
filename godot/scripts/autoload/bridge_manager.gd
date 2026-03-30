extends Node
## BridgeManager — HTTP/SSE connection to the orchestrator.
## Autoloaded singleton. Handles initial sync, live SSE updates, and write commands.

signal agent_registered(agent_data: BridgeData.AgentData)
signal agent_state_changed(agent_id: String, old_state: String, new_state: String)
signal agent_output(chunk: BridgeData.AgentOutputChunk)
signal connection_status_changed(status: String)

enum ConnectionState { DISCONNECTED, CONNECTING, CONNECTED, RECONNECTING }

@export var base_url: String = "http://localhost:8080"

var _connection_state: ConnectionState = ConnectionState.DISCONNECTED
var _agent_states: Dictionary = {}
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

var _sync_agents_request: HTTPRequest
var _sync_floors_request: HTTPRequest
var _command_request: HTTPRequest

var _agents_synced: bool = false
var _floors_synced: bool = false
var _initial_sync_failed: bool = false


func _ready() -> void:
	_sync_agents_request = HTTPRequest.new()
	add_child(_sync_agents_request)
	_sync_agents_request.request_completed.connect(_on_agents_synced)

	_sync_floors_request = HTTPRequest.new()
	add_child(_sync_floors_request)
	_sync_floors_request.request_completed.connect(_on_floors_synced)

	_command_request = HTTPRequest.new()
	add_child(_command_request)
	_command_request.request_completed.connect(_on_command_completed)

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
	_sync_agents_request.request(base_url + "/api/agents")
	_sync_floors_request.request(base_url + "/api/floors")


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
	for item: Variant in (agents_array as Array):
		if not item is Dictionary:
			continue
		var agent: BridgeData.AgentData = BridgeData.AgentData.from_dict(item as Dictionary)
		_agent_states[agent.id] = agent.state
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
	var port: int = 8080
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
			agent_registered.emit(agent)
		"agent.state_changed":
			var agent_id: String = data.get("agent_id", "")
			var new_state: String = data.get("state", "")
			var old_state: String = _agent_states.get(agent_id, "")
			agent_state_changed.emit(agent_id, old_state, new_state)
			_agent_states[agent_id] = new_state
		"agent.output":
			var chunk: BridgeData.AgentOutputChunk = BridgeData.AgentOutputChunk.from_dict(data)
			agent_output.emit(chunk)
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

func submit_task(task_id: String, priority: float) -> void:
	_enqueue_command("POST", "/api/tasks", {"task_id": task_id, "priority": priority})


func submit_dag(nodes: Array, edges: Array) -> void:
	_enqueue_command("POST", "/api/dags", {"nodes": nodes, "edges": edges})


func send_input(agent_id: String, keys: String) -> void:
	_enqueue_command("POST", "/api/agents/" + agent_id + "/input", {"keys": keys})


func get_agent_output(agent_id: String, lines: int = 50) -> void:
	_enqueue_command("GET", "/api/agents/" + agent_id + "/output?lines=" + str(lines), {})


func _enqueue_command(method: String, path: String, body: Dictionary) -> void:
	_command_queue.append({"method": method, "path": path, "body": body})
	if not _command_in_flight:
		_process_next_command()


func _process_next_command() -> void:
	if _command_queue.is_empty():
		return
	var cmd: Dictionary = _command_queue.pop_front()
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


func _on_command_completed(_result: int, _code: int, _headers: PackedStringArray, _body: PackedByteArray) -> void:
	_command_in_flight = false
	_process_next_command()
