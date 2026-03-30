# bridge_data.gd — Typed data models for the orchestrator bridge.

class_name BridgeData


class AgentData extends RefCounted:
	## Agent unique identifier.
	var id: String = ""
	## One of: idle, assigned, working, reporting, crashed.
	var state: String = "idle"
	var current_task_id: String = ""
	var last_heartbeat: int = 0
	var registered_at: int = 0

	static func from_dict(d: Dictionary) -> AgentData:
		var a := AgentData.new()
		a.id = d.get("id", "")
		a.state = d.get("state", "idle")
		a.current_task_id = d.get("current_task_id", "")
		a.last_heartbeat = d.get("last_heartbeat", 0)
		a.registered_at = d.get("registered_at", 0)
		return a


class FloorData extends RefCounted:
	var name: String = ""
	var agent_count: int = 0

	static func from_dict(d: Dictionary) -> FloorData:
		var f := FloorData.new()
		f.name = d.get("name", "")
		f.agent_count = d.get("agent_count", 0)
		return f


class AgentOutputChunk extends RefCounted:
	var agent_id: String = ""
	var payload: String = ""
	var timestamp: int = 0

	static func from_dict(d: Dictionary) -> AgentOutputChunk:
		var c := AgentOutputChunk.new()
		c.agent_id = d.get("agent_id", "")
		c.payload = d.get("payload", "")
		c.timestamp = d.get("timestamp", 0)
		return c
