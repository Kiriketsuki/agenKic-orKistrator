# bridge_data.gd — Typed data models for the orchestrator bridge.

class_name BridgeData


class AgentData extends RefCounted:
	## Agent unique identifier.
	var id: String = ""
	## Fantasy display name chosen by the spawn endpoint. Empty for an agent
	## that registered straight through gRPC. Use display_name to read it.
	var name: String = ""
	## One of: idle, assigned, working, reporting, crashed.
	var state: String = "idle"
	var current_task_id: String = ""
	var last_heartbeat: int = 0
	var registered_at: int = 0
	## Floor this agent belongs to. Empty means "assign to default floor".
	var floor_name: String = ""
	## Numeric tower floor index the agent spawned on, from the Grimoire
	## Summoning floor field (F3). 0 means no floor was recorded.
	var floor: int = 0
	## One of: alchemist, scribe, archmage, wardkeeper, librarian, enchanter, apprentice.
	var character_class: String = "apprentice"
	## LLM provider: claude, gemini, openai, ollama, deepseek, or empty.
	var provider: String = ""

	static func from_dict(d: Dictionary) -> AgentData:
		var a := AgentData.new()
		a.id = d.get("id", "")
		a.name = d.get("name", "")
		a.state = d.get("state", "idle")
		a.current_task_id = d.get("current_task_id", "")
		a.last_heartbeat = d.get("last_heartbeat", 0)
		a.registered_at = d.get("registered_at", 0)
		a.floor_name = d.get("floor_name", "")
		a.floor = d.get("floor", 0)
		a.character_class = d.get("character_class", "apprentice")
		a.provider = d.get("provider", "")
		return a

	## Returns the name the UI shows for this agent. The orchestrator does not
	## name every agent, so this falls back to the raw UUID.
	func display_name() -> String:
		return name if not name.is_empty() else id


class FloorData extends RefCounted:
	var name: String = ""
	var agent_count: int = 0
	var is_permanent: bool = false
	var polygon_sides: int = 6

	static func from_dict(d: Dictionary) -> FloorData:
		var f := FloorData.new()
		f.name = d.get("name", "")
		f.agent_count = d.get("agent_count", 0)
		f.is_permanent = d.get("is_permanent", false)
		f.polygon_sides = d.get("polygon_sides", 6)
		return f


class AgentOutputChunk extends RefCounted:
	var agent_id: String = ""
	var payload: String = ""
	var timestamp: int = 0
	## Per-output provider override. Empty = use agent default.
	var provider: String = ""
	## Orchestrator hint: true = always show as rune.
	var significant: bool = false

	static func from_dict(d: Dictionary) -> AgentOutputChunk:
		var c := AgentOutputChunk.new()
		c.agent_id = d.get("agent_id", "")
		c.payload = d.get("payload", "")
		c.timestamp = d.get("timestamp", 0)
		c.provider = d.get("provider", "")
		c.significant = d.get("significant", false)
		return c
