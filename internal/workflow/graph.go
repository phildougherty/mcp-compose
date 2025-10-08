package workflow

import "fmt"

type Graph struct {
	nodes    map[string]bool
	edges    map[string][]string
	workflow *Workflow
}

func NewGraph(workflow *Workflow) *Graph {
	g := &Graph{
		nodes:    make(map[string]bool),
		edges:    make(map[string][]string),
		workflow: workflow,
	}

	for _, node := range workflow.Nodes {
		g.nodes[node.ID] = true
	}

	for _, edge := range workflow.Edges {
		g.edges[edge.Source] = append(g.edges[edge.Source], edge.Target)
	}

	return g
}

func (g *Graph) DetectCycles() [][]string {
	var cycles [][]string

	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for nodeID := range g.nodes {
		if !visited[nodeID] {
			path := []string{}
			if cycle := g.dfs(nodeID, visited, recStack, path); cycle != nil {
				cycles = append(cycles, cycle)
			}
		}
	}

	return cycles
}

func (g *Graph) dfs(nodeID string, visited, recStack map[string]bool, path []string) []string {
	visited[nodeID] = true
	recStack[nodeID] = true
	path = append(path, nodeID)

	for _, neighbor := range g.edges[nodeID] {
		if !visited[neighbor] {
			if cycle := g.dfs(neighbor, visited, recStack, path); cycle != nil {
				return cycle
			}
		} else if recStack[neighbor] {
			cycleStart := -1
			for i, node := range path {
				if node == neighbor {
					cycleStart = i

					break
				}
			}

			if cycleStart >= 0 {
				return append(path[cycleStart:], neighbor)
			}
		}
	}

	recStack[nodeID] = false

	return nil
}

func (g *Graph) TopologicalSort() ([]string, error) {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	stack := []string{}

	var visit func(string) error
	visit = func(nodeID string) error {
		if recStack[nodeID] {
			return &CycleError{NodeID: nodeID}
		}

		if visited[nodeID] {
			return nil
		}

		visited[nodeID] = true
		recStack[nodeID] = true

		for _, neighbor := range g.edges[nodeID] {
			if err := visit(neighbor); err != nil {
				return err
			}
		}

		recStack[nodeID] = false
		stack = append([]string{nodeID}, stack...)

		return nil
	}

	for nodeID := range g.nodes {
		if !visited[nodeID] {
			if err := visit(nodeID); err != nil {
				return nil, err
			}
		}
	}

	return stack, nil
}

func (g *Graph) GetTriggerNodes() []string {
	var triggers []string

	for _, node := range g.workflow.Nodes {
		if node.Type == NodeTypeTrigger {
			triggers = append(triggers, node.ID)
		}
	}

	return triggers
}

func (g *Graph) GetReachableNodes(startNodeID string) map[string]bool {
	reachable := make(map[string]bool)
	visited := make(map[string]bool)

	var dfs func(string)
	dfs = func(nodeID string) {
		if visited[nodeID] {
			return
		}

		visited[nodeID] = true
		reachable[nodeID] = true

		for _, neighbor := range g.edges[nodeID] {
			dfs(neighbor)
		}
	}

	dfs(startNodeID)

	return reachable
}

func (g *Graph) FindUnreachableNodes() []string {
	triggerNodes := g.GetTriggerNodes()
	if len(triggerNodes) == 0 {
		return nil
	}

	allReachable := make(map[string]bool)

	for _, trigger := range triggerNodes {
		reachable := g.GetReachableNodes(trigger)
		for nodeID := range reachable {
			allReachable[nodeID] = true
		}
	}

	var unreachable []string
	for nodeID := range g.nodes {
		if !allReachable[nodeID] {
			unreachable = append(unreachable, nodeID)
		}
	}

	return unreachable
}

type CycleError struct {
	NodeID string
}

func (e *CycleError) Error() string {
	return "cycle detected at node: " + e.NodeID
}

func BuildDependencyGraph(workflow *Workflow) map[string][]string {
	graph := make(map[string][]string)

	for _, edge := range workflow.Edges {
		graph[edge.Target] = append(graph[edge.Target], edge.Source)
	}

	return graph
}

func TopologicalSort(graph map[string][]string) ([]string, error) {
	visited := make(map[string]bool)
	temp := make(map[string]bool)
	result := []string{}

	var visit func(string) error
	visit = func(node string) error {
		if temp[node] {
			return fmt.Errorf("cycle detected at node %s", node)
		}

		if visited[node] {
			return nil
		}

		temp[node] = true

		for _, dep := range graph[node] {
			if err := visit(dep); err != nil {
				return err
			}
		}

		temp[node] = false
		visited[node] = true
		result = append(result, node)

		return nil
	}

	allNodes := make(map[string]bool)
	for node := range graph {
		allNodes[node] = true
	}

	for _, deps := range graph {
		for _, dep := range deps {
			allNodes[dep] = true
		}
	}

	for node := range allNodes {
		if !visited[node] {
			if err := visit(node); err != nil {
				return nil, err
			}
		}
	}

	return result, nil
}
