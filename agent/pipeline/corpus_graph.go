package pipeline

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Y4NN777/7review/agent/review"
)

type CorpusGraph struct {
	Nodes          []GraphNode
	Edges          map[int][]GraphEdge
	SectionIndex   map[string]int
	IDIndex        map[string][]int
	RouteIndex     map[string][]int
	InterfaceIndex map[string][]int
	DataIndex      map[string][]int
	ComponentIndex map[string][]int
	TermIndex      map[string][]int
}

type GraphNode struct {
	Index      int
	Section    corpusSection
	IDs        map[string]string
	Routes     map[string]struct{}
	Interfaces map[string]struct{}
	Data       map[string]struct{}
	Components map[string]struct{}
	Terms      map[string]struct{}
}

type GraphEdge struct {
	From  int
	To    int
	Type  string
	Label string
}

func buildCorpusGraph(sections []corpusSection) CorpusGraph {
	graph := CorpusGraph{
		Nodes:          make([]GraphNode, 0, len(sections)),
		Edges:          make(map[int][]GraphEdge),
		SectionIndex:   make(map[string]int),
		IDIndex:        make(map[string][]int),
		RouteIndex:     make(map[string][]int),
		InterfaceIndex: make(map[string][]int),
		DataIndex:      make(map[string][]int),
		ComponentIndex: make(map[string][]int),
		TermIndex:      make(map[string][]int),
	}
	for i, section := range sections {
		node := buildGraphNode(i, section)
		graph.Nodes = append(graph.Nodes, node)
		graph.SectionIndex[sectionKey(section)] = i
		for id := range node.IDs {
			graph.IDIndex[id] = appendUniqueInt(graph.IDIndex[id], i)
		}
		for route := range node.Routes {
			graph.RouteIndex[route] = appendUniqueInt(graph.RouteIndex[route], i)
		}
		for value := range node.Interfaces {
			graph.InterfaceIndex[value] = appendUniqueInt(graph.InterfaceIndex[value], i)
		}
		for value := range node.Data {
			graph.DataIndex[value] = appendUniqueInt(graph.DataIndex[value], i)
		}
		for value := range node.Components {
			graph.ComponentIndex[value] = appendUniqueInt(graph.ComponentIndex[value], i)
		}
		for value := range node.Terms {
			graph.TermIndex[value] = appendUniqueInt(graph.TermIndex[value], i)
		}
	}
	graph.addHierarchyEdges()
	graph.addIDTraceEdges()
	graph.addSharedFeatureEdges(graph.RouteIndex, "interface_trace")
	graph.addSharedFeatureEdges(graph.InterfaceIndex, "interface_trace")
	graph.addSharedFeatureEdges(graph.DataIndex, "data_trace")
	graph.addComponentTraceEdges()
	graph.addOwnershipEdges()
	graph.sortEdges()
	return graph
}

func buildGraphNode(index int, section corpusSection) GraphNode {
	text := section.Path + "\n" + section.Title + "\n" + section.Content
	node := GraphNode{
		Index:      index,
		Section:    section,
		IDs:        make(map[string]string),
		Routes:     make(map[string]struct{}),
		Interfaces: make(map[string]struct{}),
		Data:       make(map[string]struct{}),
		Components: make(map[string]struct{}),
		Terms:      setFromStrings(tokenizeTerms(text)),
	}
	idClass := classifyGraphDocumentContext(section)
	for _, id := range idPattern.FindAllString(text, -1) {
		id = strings.ToUpper(id)
		if validCorpusID(id) {
			node.IDs[id] = idClass
		}
	}
	for _, route := range routePattern.FindAllString(text, -1) {
		route = normalizeRoute(route)
		if apiRouteSignal(route) || strings.Contains(section.Title, route) {
			node.Routes[route] = struct{}{}
			node.Interfaces[strings.ToLower(route)] = struct{}{}
		}
	}
	for _, value := range graphInterfaceNames(section, text) {
		node.Interfaces[value] = struct{}{}
	}
	for _, value := range graphDataNames(section, text) {
		node.Data[value] = struct{}{}
	}
	for _, value := range graphComponentNames(section, text) {
		node.Components[value] = struct{}{}
	}
	return node
}

func classifyGraphDocumentContext(section corpusSection) string {
	lower := strings.ToLower(filepath.ToSlash(section.Path) + "\n" + section.Title + "\n" + section.Authority)
	switch {
	case strings.Contains(lower, "openapi"), strings.Contains(lower, "asyncapi"), section.Kind == review.KindAPI:
		return "interface"
	case strings.Contains(lower, "data-model"), strings.Contains(lower, "datamodel"), strings.Contains(lower, "data_model"), strings.Contains(lower, "schema"):
		return "data"
	case strings.Contains(lower, "srs"), strings.Contains(lower, "prd"), strings.Contains(lower, "requirement"), section.Authority == "requirements":
		return "requirement"
	case strings.Contains(lower, "contract"), strings.Contains(lower, "rules"), strings.Contains(lower, "rule"), strings.Contains(lower, "security"), section.Kind == review.KindSecurity:
		return "constraint"
	case strings.Contains(lower, "adr"), strings.Contains(lower, "decision"), strings.Contains(lower, "architecture"), section.Kind == review.KindArchitecture:
		return "decision"
	case strings.Contains(lower, "design"), section.Kind == review.KindDesign:
		return "design"
	default:
		return "reference"
	}
}

func graphInterfaceNames(section corpusSection, text string) []string {
	values := make(map[string]struct{})
	title := strings.ToLower(section.Title)
	if strings.HasPrefix(title, "schemas.") || strings.HasPrefix(title, "messages.") || strings.HasPrefix(title, "channels.") {
		values[normalizeGraphName(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(title, "schemas."), "messages."), "channels."))] = struct{}{}
	}
	if strings.HasPrefix(title, "paths.") {
		if route := normalizeRoute(strings.TrimPrefix(section.Title, "paths.")); route != "" {
			values[strings.ToLower(route)] = struct{}{}
		}
	}
	for _, match := range schemaRefPattern.FindAllStringSubmatch(text, -1) {
		values[normalizeGraphName(match[1])] = struct{}{}
	}
	for _, value := range structuralKeyNames(section) {
		values[value] = struct{}{}
	}
	return sortedSet(values)
}

func graphDataNames(section corpusSection, text string) []string {
	values := make(map[string]struct{})
	lowerTitle := strings.ToLower(section.Title)
	if strings.HasPrefix(lowerTitle, "schemas.") {
		values[normalizeGraphName(strings.TrimPrefix(lowerTitle, "schemas."))] = struct{}{}
	}
	for _, match := range entityPattern.FindAllStringSubmatch(text, -1) {
		for _, value := range match[1:] {
			if value = normalizeGraphName(value); value != "" && !stopSignal(value) {
				values[value] = struct{}{}
			}
		}
	}
	if dataContextSection(section) {
		for _, token := range tokenizeTerms(section.Title + "\n" + section.Path) {
			if dataNameToken(token) {
				values[token] = struct{}{}
			}
		}
	}
	return sortedSet(values)
}

func dataContextSection(section corpusSection) bool {
	lower := strings.ToLower(filepath.ToSlash(section.Path) + "\n" + section.Title + "\n" + section.Authority)
	return strings.Contains(lower, "data") ||
		strings.Contains(lower, "schema") ||
		strings.Contains(lower, "model") ||
		strings.HasPrefix(strings.ToLower(section.Title), "schemas.")
}

func graphComponentNames(section corpusSection, text string) []string {
	values := make(map[string]struct{})
	for _, token := range tokenizeTerms(section.Path + "\n" + section.Title) {
		if componentNameToken(token) {
			values[token] = struct{}{}
		}
	}
	return sortedSet(values)
}

func structuralKeyNames(section corpusSection) []string {
	var raw []string
	for _, value := range strings.FieldsFunc(section.Title, func(r rune) bool {
		return r == '.' || r == '/' || r == '_' || r == '-' || r == ' '
	}) {
		if value = normalizeGraphName(value); structuralNameToken(value) {
			raw = append(raw, value)
		}
	}
	return uniqueStrings(raw)
}

func structuralNameToken(token string) bool {
	if len(token) < 4 || stopSignal(token) {
		return false
	}
	switch token {
	case "path", "paths", "schema", "schemas", "message", "messages", "component", "components", "state", "states", "admin", "list", "get", "post", "put", "patch", "delete":
		return false
	default:
		return true
	}
}

func (graph *CorpusGraph) addHierarchyEdges() {
	byDocTitle := make(map[string]int)
	firstByDoc := make(map[string]int)
	for _, node := range graph.Nodes {
		key := node.Section.Path + "\x00" + node.Section.Title
		byDocTitle[key] = node.Index
		if _, ok := firstByDoc[node.Section.Path]; !ok || node.Section.Ordinal < graph.Nodes[firstByDoc[node.Section.Path]].Section.Ordinal {
			firstByDoc[node.Section.Path] = node.Index
		}
	}
	for _, node := range graph.Nodes {
		if len(node.Section.HeadingPath) >= 2 {
			parentTitle := node.Section.HeadingPath[len(node.Section.HeadingPath)-2]
			if parent, ok := byDocTitle[node.Section.Path+"\x00"+parentTitle]; ok && parent != node.Index {
				graph.addEdge(node.Index, parent, "hierarchy", "parent "+parentTitle)
			}
		}
		if first, ok := firstByDoc[node.Section.Path]; ok && first != node.Index {
			graph.addEdge(node.Index, first, "hierarchy_overview", "overview "+graph.Nodes[first].Section.Title)
		}
	}
}

func (graph *CorpusGraph) addIDTraceEdges() {
	for id, nodes := range graph.IDIndex {
		if len(nodes) < 2 || len(nodes) > 24 {
			continue
		}
		for _, from := range nodes {
			edgeType := idTraceEdgeType(id, graph.Nodes[from].IDs[id])
			for _, to := range nodes {
				if from == to {
					continue
				}
				graph.addEdge(from, to, edgeType, id)
			}
		}
	}
}

func (graph *CorpusGraph) addSharedFeatureEdges(index map[string][]int, edgeType string) {
	for label, nodes := range index {
		if label == "" || len(nodes) < 2 || len(nodes) > 18 {
			continue
		}
		for _, from := range nodes {
			for _, to := range nodes {
				if from != to {
					graph.addEdge(from, to, edgeType, label)
				}
			}
		}
	}
}

func (graph *CorpusGraph) addComponentTraceEdges() {
	for component, nodes := range graph.ComponentIndex {
		if len(nodes) < 2 || len(nodes) > 18 {
			continue
		}
		for _, from := range nodes {
			for _, to := range nodes {
				if from == to {
					continue
				}
				if !componentTracePair(graph.Nodes[from].Section, graph.Nodes[to].Section) {
					continue
				}
				graph.addEdge(from, to, "ui_trace", component)
			}
		}
	}
}

func (graph *CorpusGraph) addOwnershipEdges() {
	for component, nodes := range graph.ComponentIndex {
		if len(nodes) < 2 || len(nodes) > 18 {
			continue
		}
		var owners []int
		for _, idx := range nodes {
			if ownershipSection(graph.Nodes[idx].Section) {
				owners = append(owners, idx)
			}
		}
		for _, owner := range owners {
			for _, node := range nodes {
				if owner != node {
					graph.addEdge(node, owner, "ownership_trace", component)
				}
			}
		}
	}
}

func (graph *CorpusGraph) addEdge(from, to int, edgeType, label string) {
	edge := GraphEdge{From: from, To: to, Type: edgeType, Label: label}
	for _, existing := range graph.Edges[from] {
		if existing.To == edge.To && existing.Type == edge.Type && existing.Label == edge.Label {
			return
		}
	}
	graph.Edges[from] = append(graph.Edges[from], edge)
}

func (graph *CorpusGraph) sortEdges() {
	for from := range graph.Edges {
		sort.SliceStable(graph.Edges[from], func(i, j int) bool {
			a := graph.Edges[from][i]
			b := graph.Edges[from][j]
			if a.Type == b.Type {
				if a.Label == b.Label {
					return graph.Nodes[a.To].Section.Title < graph.Nodes[b.To].Section.Title
				}
				return a.Label < b.Label
			}
			return graphEdgeRank(a.Type) < graphEdgeRank(b.Type)
		})
	}
}

func idTraceEdgeType(id, class string) string {
	prefix, _, _ := strings.Cut(strings.ToUpper(id), "-")
	switch {
	case prefix == "ADR":
		return "decision_trace"
	case prefix == "UC" || prefix == "OPC":
		return "operation_trace"
	case prefix == "REQ" || prefix == "FR" || prefix == "NFR":
		return "requirement_trace"
	case prefix == "INV" || prefix == "LAW" || prefix == "RULE" || prefix == "PRO" || prefix == "CTRL":
		return "constraint_trace"
	case class == "requirement":
		return "requirement_trace"
	case class == "constraint":
		return "constraint_trace"
	case class == "decision":
		return "decision_trace"
	case class == "interface":
		return "interface_trace"
	case class == "data":
		return "data_trace"
	default:
		return "operation_trace"
	}
}

func graphEdgeRank(edgeType string) int {
	switch edgeType {
	case "requirement_trace", "constraint_trace", "operation_trace", "decision_trace":
		return 1
	case "interface_trace", "data_trace", "ui_trace", "ownership_trace":
		return 2
	case "hierarchy":
		return 3
	case "hierarchy_overview":
		return 4
	default:
		return 5
	}
}

func ownershipSection(section corpusSection) bool {
	lower := strings.ToLower(filepath.ToSlash(section.Path) + "\n" + section.Title)
	return strings.Contains(lower, "owner") ||
		strings.Contains(lower, "ownership") ||
		strings.Contains(lower, "responsibility") ||
		strings.Contains(lower, "responsability") ||
		strings.Contains(lower, "component")
}

func dataNameToken(token string) bool {
	return strings.Contains(token, "_") || strings.HasSuffix(token, "s") || strings.Contains(token, "schema") || strings.Contains(token, "model")
}

func componentNameToken(token string) bool {
	if len(token) < 4 || stopSignal(token) {
		return false
	}
	switch token {
	case "path", "paths", "schema", "schemas", "message", "messages", "component", "components", "state", "states", "screen", "screens", "view", "views", "merge", "request", "review", "rules", "contract":
		return false
	default:
		return true
	}
}

func componentTracePair(a, b corpusSection) bool {
	return a.Kind == review.KindDesign ||
		b.Kind == review.KindDesign ||
		ownershipSection(a) ||
		ownershipSection(b)
}

func normalizeGraphName(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.Trim(value, "`'\"()[]{}:;,")
	value = strings.TrimPrefix(value, "#/components/schemas/")
	value = strings.TrimPrefix(value, "#/components/messages/")
	return normalizeTerm(value)
}

func validCorpusID(id string) bool {
	id = strings.TrimSpace(strings.ToUpper(id))
	if id == "" {
		return false
	}
	for _, suffix := range []string{".MD", ".MARKDOWN", ".TXT", ".YAML", ".YML", ".JSON", ".PROTO"} {
		if strings.HasSuffix(id, suffix) {
			return false
		}
	}
	prefix, suffix, ok := strings.Cut(id, "-")
	if !ok || prefix == "" || suffix == "" {
		return false
	}
	if strings.ContainsAny(suffix, "0123456789") {
		return true
	}
	switch prefix {
	case "REQ", "FR", "NFR", "UC", "RULE", "INV", "LAW", "PRO", "OPC", "ADR", "CMP", "CTRL", "DSO", "GAR", "API", "SEC", "DATA", "UI":
		return true
	default:
		return false
	}
}

func appendUniqueInt(values []int, value int) []int {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func sortedSet(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

var (
	schemaRefPattern = regexp.MustCompile(`(?i)(?:#/components/(?:schemas|messages)/|schemas[./]|messages[./])([A-Za-z][A-Za-z0-9_.-]+)`)
)
