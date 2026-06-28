package pipeline

import (
	"fmt"
	"sort"
	"strings"
)

type graphSeed struct {
	Node   int
	Kind   string
	Value  string
	Score  int
	Reason string
}

type graphExpansionLimits struct {
	PerSeed int
}

func matchGraphSeeds(graph CorpusGraph, signals reviewSignals) []graphSeed {
	var seeds []graphSeed
	for id := range signals.IDs {
		for _, idx := range graph.IDIndex[id] {
			if graphSeedNodeExcluded(graph.Nodes[idx].Section) {
				continue
			}
			seeds = append(seeds, graphSeed{
				Node:   idx,
				Kind:   "identifier",
				Value:  id,
				Score:  230 + authorityScore(graph.Nodes[idx].Section),
				Reason: "seed: identifier " + id,
			})
		}
	}
	for route := range signals.Routes {
		for _, idx := range graph.RouteIndex[route] {
			if graphSeedNodeExcluded(graph.Nodes[idx].Section) {
				continue
			}
			seeds = append(seeds, graphSeed{
				Node:   idx,
				Kind:   "route",
				Value:  route,
				Score:  190 + authorityScore(graph.Nodes[idx].Section),
				Reason: "seed: API route " + route,
			})
		}
		for _, idx := range graph.InterfaceIndex[strings.ToLower(route)] {
			if graphSeedNodeExcluded(graph.Nodes[idx].Section) {
				continue
			}
			seeds = append(seeds, graphSeed{
				Node:   idx,
				Kind:   "route",
				Value:  route,
				Score:  175 + authorityScore(graph.Nodes[idx].Section),
				Reason: "seed: API route " + route,
			})
		}
	}
	for entity := range signals.Entities {
		for _, idx := range graph.DataIndex[entity] {
			if graphSeedNodeExcluded(graph.Nodes[idx].Section) {
				continue
			}
			seeds = append(seeds, graphSeed{
				Node:   idx,
				Kind:   "entity",
				Value:  entity,
				Score:  120 + authorityScore(graph.Nodes[idx].Section),
				Reason: "seed: entity " + entity,
			})
		}
	}
	for component := range signals.Components {
		if !componentNameToken(component) {
			continue
		}
		for _, idx := range graph.ComponentIndex[component] {
			if graphSeedNodeExcluded(graph.Nodes[idx].Section) {
				continue
			}
			seeds = append(seeds, graphSeed{
				Node:   idx,
				Kind:   "component",
				Value:  component,
				Score:  110 + authorityScore(graph.Nodes[idx].Section),
				Reason: "seed: component " + component,
			})
		}
	}
	sort.SliceStable(seeds, func(i, j int) bool {
		if seeds[i].Score == seeds[j].Score {
			if seeds[i].Node == seeds[j].Node {
				return seeds[i].Value < seeds[j].Value
			}
			return graph.Nodes[seeds[i].Node].Section.Path < graph.Nodes[seeds[j].Node].Section.Path
		}
		return seeds[i].Score > seeds[j].Score
	})
	return uniqueGraphSeeds(seeds)
}

func graphSeedNodeExcluded(section corpusSection) bool {
	return corpusCodeRule(section.Path) != ""
}

func mergeGraphSeedEvidence(scored []scoredCorpusSection, graph CorpusGraph, seeds []graphSeed) []scoredCorpusSection {
	selected := make(map[string]int, len(scored))
	for i, item := range scored {
		selected[sectionKey(item.section)] = i
	}
	for _, seed := range seeds {
		if seed.Node < 0 || seed.Node >= len(graph.Nodes) {
			continue
		}
		section := graph.Nodes[seed.Node].Section
		key := sectionKey(section)
		if idx, ok := selected[key]; ok {
			if scored[idx].score < seed.Score {
				scored[idx].score = seed.Score
			}
			scored[idx].matchedSignals = uniqueStrings(append(scored[idx].matchedSignals, seed.Value))
			if scored[idx].reason == "" {
				scored[idx].reason = seed.Reason
			}
			continue
		}
		selected[key] = len(scored)
		scored = append(scored, scoredCorpusSection{
			section:        section,
			score:          seed.Score,
			matchedSignals: []string{seed.Value},
			reason:         seed.Reason,
		})
	}
	return scored
}

func expandGraphEvidence(graph CorpusGraph, scored []scoredCorpusSection, seeds []graphSeed, limits graphExpansionLimits) []scoredCorpusSection {
	if len(seeds) == 0 || len(graph.Nodes) == 0 {
		return scored
	}
	perSeed := limits.PerSeed
	if perSeed <= 0 {
		perSeed = 2
	}
	out := append([]scoredCorpusSection(nil), scored...)
	selected := make(map[string]struct{}, len(out))
	for _, item := range out {
		selected[sectionKey(item.section)] = struct{}{}
	}
	for _, seed := range seeds {
		added := 0
		for _, candidate := range graphCandidatesForSeed(graph, seed) {
			if added >= perSeed {
				break
			}
			if candidate.node < 0 || candidate.node >= len(graph.Nodes) {
				continue
			}
			section := graph.Nodes[candidate.node].Section
			if corpusCodeRule(section.Path) != "" {
				continue
			}
			key := sectionKey(section)
			if _, ok := selected[key]; ok {
				continue
			}
			selected[key] = struct{}{}
			added++
			out = append(out, scoredCorpusSection{
				section:        section,
				score:          graphRelatedScore(seed.Score, candidate.depth, section),
				matchedSignals: []string{seed.Value},
				reason:         graphSelectionReason(graph, seed, candidate),
			})
		}
	}
	return out
}

func expandGraphHierarchyEvidence(graph CorpusGraph, scored []scoredCorpusSection, limits graphExpansionLimits) []scoredCorpusSection {
	if len(scored) == 0 || len(graph.Nodes) == 0 {
		return scored
	}
	perSeed := limits.PerSeed
	if perSeed <= 0 {
		perSeed = 2
	}
	out := append([]scoredCorpusSection(nil), scored...)
	selected := make(map[string]struct{}, len(out))
	for _, item := range out {
		selected[sectionKey(item.section)] = struct{}{}
	}
	relatedBySource := make(map[string]int)
	for _, item := range scored {
		if item.score < 20 || !relatedContextEligible(item.section) {
			continue
		}
		node, ok := graph.SectionIndex[sectionKey(item.section)]
		if !ok {
			continue
		}
		added := 0
		for _, candidate := range hierarchyCandidatesForNode(graph, node) {
			if added >= perSeed {
				break
			}
			section := graph.Nodes[candidate.node].Section
			if relatedBySource[section.Path] >= 2 {
				continue
			}
			key := sectionKey(section)
			if _, ok := selected[key]; ok {
				continue
			}
			selected[key] = struct{}{}
			relatedBySource[section.Path]++
			added++
			out = append(out, scoredCorpusSection{
				section:        section,
				score:          hierarchyRelatedScore(item.score, candidate.depth, section),
				matchedSignals: append([]string(nil), item.matchedSignals...),
				reason:         graphSelectionReason(graph, graphSeed{Node: node}, candidate),
			})
		}
	}
	return out
}

type graphCandidate struct {
	node  int
	edge  GraphEdge
	depth int
	score int
}

func graphCandidatesForSeed(graph CorpusGraph, seed graphSeed) []graphCandidate {
	type queueItem struct {
		node  int
		depth int
	}
	visited := map[int]int{seed.Node: 0}
	queue := []queueItem{{node: seed.Node}}
	best := make(map[int]graphCandidate)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, edge := range graph.Edges[current.node] {
			nextDepth := current.depth + 1
			if !graphEdgeAllowed(edge.Type, nextDepth) {
				continue
			}
			if previous, ok := visited[edge.To]; ok && previous <= nextDepth {
				continue
			}
			visited[edge.To] = nextDepth
			candidate := graphCandidate{
				node:  edge.To,
				edge:  edge,
				depth: nextDepth,
				score: graphCandidateScore(edge, nextDepth),
			}
			if existing, ok := best[edge.To]; !ok || candidate.score > existing.score {
				best[edge.To] = candidate
			}
			if (edge.Type == "hierarchy" || edge.Type == "hierarchy_overview") && nextDepth < 2 {
				queue = append(queue, queueItem{node: edge.To, depth: nextDepth})
			}
		}
	}
	out := make([]graphCandidate, 0, len(best))
	for _, candidate := range best {
		out = append(out, candidate)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score == out[j].score {
			if out[i].depth == out[j].depth {
				return graph.Nodes[out[i].node].Section.Path < graph.Nodes[out[j].node].Section.Path
			}
			return out[i].depth < out[j].depth
		}
		return out[i].score > out[j].score
	})
	return out
}

func hierarchyCandidatesForNode(graph CorpusGraph, node int) []graphCandidate {
	type queueItem struct {
		node  int
		depth int
	}
	visited := map[int]int{node: 0}
	queue := []queueItem{{node: node}}
	var out []graphCandidate
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, edge := range graph.Edges[current.node] {
			if edge.Type != "hierarchy" {
				continue
			}
			nextDepth := current.depth + 1
			if nextDepth > 2 {
				continue
			}
			if previous, ok := visited[edge.To]; ok && previous <= nextDepth {
				continue
			}
			visited[edge.To] = nextDepth
			out = append(out, graphCandidate{
				node:  edge.To,
				edge:  edge,
				depth: nextDepth,
				score: graphCandidateScore(edge, nextDepth),
			})
			if nextDepth < 2 {
				queue = append(queue, queueItem{node: edge.To, depth: nextDepth})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score == out[j].score {
			if out[i].depth == out[j].depth {
				return graph.Nodes[out[i].node].Section.Ordinal < graph.Nodes[out[j].node].Section.Ordinal
			}
			return out[i].depth < out[j].depth
		}
		return out[i].score > out[j].score
	})
	return out
}

func graphEdgeAllowed(edgeType string, depth int) bool {
	if edgeType == "hierarchy" || edgeType == "hierarchy_overview" {
		return depth <= 2
	}
	return depth <= 1
}

func graphCandidateScore(edge GraphEdge, depth int) int {
	score := 95 - (depth-1)*20
	switch edge.Type {
	case "requirement_trace", "constraint_trace", "operation_trace", "decision_trace":
		score += 35
	case "interface_trace", "data_trace":
		score += 25
	case "ui_trace", "ownership_trace":
		score += 20
	case "hierarchy":
		score += 5
	case "hierarchy_overview":
		score -= 15
	}
	return score
}

func graphRelatedScore(seedScore, depth int, section corpusSection) int {
	score := seedScore/2 + authorityScore(section)
	if depth > 1 {
		score -= 15
	}
	if score < 55 {
		return 55
	}
	if score > 135 {
		return 135
	}
	return score
}

func hierarchyRelatedScore(seedScore, depth int, section corpusSection) int {
	score := relatedScore(seedScore, section)
	if depth > 1 {
		score -= 10
	}
	if score < 70 {
		return 70
	}
	if score > 95 {
		return 95
	}
	return score
}

func graphSelectionReason(graph CorpusGraph, seed graphSeed, candidate graphCandidate) string {
	from := graph.Nodes[candidate.edge.From].Section
	to := graph.Nodes[candidate.edge.To].Section
	label := candidate.edge.Label
	switch candidate.edge.Type {
	case "hierarchy":
		return fmt.Sprintf("hierarchy: %s selected with %s#%s", label, from.Path, from.Title)
	case "hierarchy_overview":
		return fmt.Sprintf("hierarchy: %s selected with %s#%s", label, from.Path, from.Title)
	case "interface_trace":
		return fmt.Sprintf("interface_trace: %s -> %s#%s", label, to.Path, to.Title)
	case "data_trace":
		return fmt.Sprintf("data_trace: %s shared with %s#%s", label, from.Path, from.Title)
	case "ui_trace":
		return fmt.Sprintf("ui_trace: %s shared with %s#%s", label, from.Path, from.Title)
	case "ownership_trace":
		return fmt.Sprintf("ownership_trace: %s -> %s#%s", label, to.Path, to.Title)
	default:
		return fmt.Sprintf("%s: %s shared with %s#%s", candidate.edge.Type, label, from.Path, from.Title)
	}
}

func uniqueGraphSeeds(seeds []graphSeed) []graphSeed {
	seen := make(map[string]struct{}, len(seeds))
	out := make([]graphSeed, 0, len(seeds))
	for _, seed := range seeds {
		key := fmt.Sprintf("%d\x00%s\x00%s", seed.Node, seed.Kind, strings.ToLower(seed.Value))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, seed)
	}
	return out
}
