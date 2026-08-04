class_name ProviderRoster
## ProviderRoster — parses the GET /api/providers response and applies the
## user's sigil config (enabled set, order) to build the Grimoire flyout's
## sigil grid (F3 Grimoire Summoning).
##
## The bridge is the single source of truth for the kind list (see
## handlers.go's providerRoster doc-comment): a new adapter appears here with
## no GUI change. This class only filters and orders what the bridge already
## sent.

## Parses the raw JSON response body of GET /api/providers into an
## Array[Dictionary], each entry {kind, display, accent}. Tolerates a missing
## or malformed body by returning an empty array, so a parse failure never
## crashes the flyout.
static func parse_response(parsed: Variant) -> Array:
	var providers: Array = []
	if not (parsed is Dictionary):
		return providers
	var raw_list: Variant = (parsed as Dictionary).get("providers", null)
	if not (raw_list is Array):
		return providers
	for item: Variant in (raw_list as Array):
		if not (item is Dictionary):
			continue
		var d: Dictionary = item as Dictionary
		var kind: String = String(d.get("kind", ""))
		if kind.is_empty():
			continue
		providers.append({
			"kind": kind,
			"display": String(d.get("display", kind)),
			"accent": String(d.get("accent", "#ffffff")),
		})
	return providers


## Filters `providers` to the enabled kinds in `enabled` (a PackedStringArray
## or Array of kind strings), then orders the result by `order` (a list of
## kind strings — entries absent from `order` sort after the ordered ones, in
## their roster-arrival order). An empty `enabled` means "show everything" —
## a freshly-installed config with no explicit toggles must not hide the
## whole grid.
static func filtered_and_ordered(providers: Array, enabled: Array, order: Array) -> Array:
	var kept: Array = providers
	if not enabled.is_empty():
		kept = providers.filter(func(p: Dictionary) -> bool: return enabled.has(p["kind"]))
	var by_kind: Dictionary = {}
	for p: Dictionary in kept:
		by_kind[p["kind"]] = p
	var result: Array = []
	for kind: String in order:
		if by_kind.has(kind):
			result.append(by_kind[kind])
			by_kind.erase(kind)
	for p: Dictionary in kept:
		if by_kind.has(p["kind"]):
			result.append(p)
			by_kind.erase(p["kind"])
	return result
