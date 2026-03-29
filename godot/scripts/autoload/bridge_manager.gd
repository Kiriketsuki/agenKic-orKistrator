extends Node
## BridgeManager — HTTP/SSE connection to the orchestrator.
## Autoloaded singleton. Handles initial sync (GET) and live updates (SSE).

signal agent_registered(agent_id: String, data: Dictionary)
signal agent_deregistered(agent_id: String)
signal agent_state_changed(agent_id: String, old_state: String, new_state: String)
signal agent_output(agent_id: String, line: String, significant: bool)
signal floor_load_updated(project_id: String, composite_load: float)
signal floor_created(project_id: String, is_permanent: bool)
signal floor_removed(project_id: String)
signal connection_status_changed(status: String)

@export var base_url: String = "http://localhost:8080"

var _http_client: HTTPClient
var _sse_buffer: String = ""
var _connected: bool = false


func _ready() -> void:
	pass


func _process(_delta: float) -> void:
	pass
