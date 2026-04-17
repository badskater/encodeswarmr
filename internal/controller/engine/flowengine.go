package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/badskater/encodeswarmr/internal/db"
)

// errCyclicDAG is returned by ValidateFlow when a back-edge is detected.
// It is also used internally by ExecuteFlow to surface the same condition.
var errCyclicDAG = fmt.Errorf("flowengine: graph contains a cycle")

// FlowEngine interprets a flow graph and translates it into an ordered list
// of TaskSteps that can be handed to the job expander.
type FlowEngine struct {
	store  db.Store
	logger *slog.Logger
}

// NewFlowEngine creates a FlowEngine.
func NewFlowEngine(store db.Store, logger *slog.Logger) *FlowEngine {
	return &FlowEngine{store: store, logger: logger}
}

// TaskStep describes one unit of work derived from a flow graph node.
type TaskStep struct {
	// NodeID is the ID of the graph node that produced this step.
	NodeID string
	// NodeType is the type string from the node (e.g. "input_source",
	// "encode", "audio", "analysis", "condition", "webhook", "output").
	NodeType string
	// Config holds the node's configuration key/value pairs.
	Config map[string]any
	// DependsOn lists node IDs whose steps must complete before this one.
	DependsOn []string
}

// flowGraph is the parsed representation of a flow's graph JSON.
type flowGraph struct {
	Nodes []flowNode `json:"nodes"`
	Edges []flowEdge `json:"edges"`
}

// flowNode is a single node in the flow graph.
type flowNode struct {
	ID   string         `json:"id"`
	Type string         `json:"type"`
	Data map[string]any `json:"data"`
}

// flowEdge connects a source node to a target node.
// Handle distinguishes true/false branches for condition nodes.
type flowEdge struct {
	ID           string `json:"id"`
	Source       string `json:"source"`
	Target       string `json:"target"`
	SourceHandle string `json:"sourceHandle"` // "true" | "false" | "" for condition nodes
}

// ValidateFlow parses the flow graph and checks for structural problems:
//   - No nodes
//   - No input_ node
//   - Edge targets that reference non-existent nodes
//   - Cycles (back-edges in DFS)
//
// It does not execute the graph or evaluate conditions.  Call this when a
// flow definition is saved so problems are surfaced early rather than at
// expansion time.
func (fe *FlowEngine) ValidateFlow(flow *db.Flow) error {
	var g flowGraph
	if err := json.Unmarshal(flow.Graph, &g); err != nil {
		return fmt.Errorf("flowengine: validate flow %s: parse graph: %w", flow.ID, err)
	}
	if len(g.Nodes) == 0 {
		return fmt.Errorf("flowengine: validate flow %s: no nodes", flow.ID)
	}

	nodeByID := make(map[string]flowNode, len(g.Nodes))
	for _, n := range g.Nodes {
		nodeByID[n.ID] = n
	}

	// Verify all edge targets and sources exist.
	for _, e := range g.Edges {
		if _, ok := nodeByID[e.Source]; !ok {
			return fmt.Errorf("flowengine: validate flow %s: edge %s references unknown source node %s", flow.ID, e.ID, e.Source)
		}
		if _, ok := nodeByID[e.Target]; !ok {
			return fmt.Errorf("flowengine: validate flow %s: edge %s references unknown target node %s", flow.ID, e.ID, e.Target)
		}
	}

	hasInput := false
	for _, n := range g.Nodes {
		if len(n.Type) >= 6 && n.Type[:6] == "input_" {
			hasInput = true
			break
		}
	}
	if !hasInput {
		return fmt.Errorf("flowengine: validate flow %s: no input node", flow.ID)
	}

	// Cycle detection via DFS with three-color marking.
	// white (absent) → grey (in stack) → black (done)
	type color int
	const (
		white color = iota
		grey
		black
	)
	mark := make(map[string]color, len(g.Nodes))

	edgesFrom := make(map[string][]flowEdge, len(g.Edges))
	for _, e := range g.Edges {
		edgesFrom[e.Source] = append(edgesFrom[e.Source], e)
	}

	var dfs func(id string) error
	dfs = func(id string) error {
		if mark[id] == black {
			return nil
		}
		if mark[id] == grey {
			return fmt.Errorf("%w: back-edge detected at node %s", errCyclicDAG, id)
		}
		mark[id] = grey
		for _, edge := range edgesFrom[id] {
			if err := dfs(edge.Target); err != nil {
				return err
			}
		}
		mark[id] = black
		return nil
	}

	for _, n := range g.Nodes {
		if err := dfs(n.ID); err != nil {
			return fmt.Errorf("flowengine: validate flow %s: %w", flow.ID, err)
		}
	}
	return nil
}

// ExecuteFlow parses the flow graph stored in flow, finds the input node,
// walks the edges in order, evaluates condition nodes, and returns the
// resulting ordered list of TaskSteps.  The sourceID is passed through to
// any input node's config so callers can map it back to a source record.
//
// Condition nodes are evaluated against live source analysis data when
// available, so branches are resolved at expansion time rather than deferred.
//
// Error-edge routing: if a node's data map contains a key "simulate_error"
// with value true (bool) or "true" (string), the engine treats that node as
// failed and follows any outgoing edge whose SourceHandle is "error" instead
// of the normal successor edges.  This lets flow authors wire an
// error-handler sub-graph without changing the TaskStep public API.
//
// Context cancellation: walk checks ctx.Done() before visiting each node and
// returns ctx.Err() immediately, so the caller gets a clean error instead of
// a partial step list.
//
// Panics inside walk are recovered, logged, and surfaced as errors so the
// engine goroutine is never silently killed.
func (fe *FlowEngine) ExecuteFlow(ctx context.Context, flow *db.Flow, sourceID string) (steps []TaskStep, retErr error) {
	// Panic recovery: convert any panic inside ExecuteFlow to an error so the
	// engine background loop remains alive.
	defer func() {
		if r := recover(); r != nil {
			fe.logger.Error("flowengine: panic in ExecuteFlow",
				"flow_id", flow.ID,
				"panic", fmt.Sprintf("%v", r),
			)
			retErr = fmt.Errorf("flowengine: panic in flow %s: %v", flow.ID, r)
			steps = nil
		}
	}()

	// 1. Parse graph JSON.
	var g flowGraph
	if err := json.Unmarshal(flow.Graph, &g); err != nil {
		return nil, fmt.Errorf("flowengine: parse graph for flow %s: %w", flow.ID, err)
	}

	// Load source analysis summary for condition evaluation.
	analysisSummary := map[string]any{}
	if ars, err := fe.store.ListAnalysisResults(ctx, sourceID); err == nil {
		for _, ar := range ars {
			if len(ar.Summary) > 0 {
				_ = json.Unmarshal(ar.Summary, &analysisSummary)
				break
			}
		}
	}

	if len(g.Nodes) == 0 {
		return nil, fmt.Errorf("flowengine: flow %s has no nodes", flow.ID)
	}

	// Build lookup maps for fast traversal.
	nodeByID := make(map[string]flowNode, len(g.Nodes))
	for _, n := range g.Nodes {
		nodeByID[n.ID] = n
	}

	// edgesFrom maps a source node ID → slice of outgoing edges.
	edgesFrom := make(map[string][]flowEdge, len(g.Edges))
	for _, e := range g.Edges {
		edgesFrom[e.Source] = append(edgesFrom[e.Source], e)
	}

	// 2. Find the input node (type starts with "input_").
	var inputNode *flowNode
	for i := range g.Nodes {
		if len(g.Nodes[i].Type) >= 6 && g.Nodes[i].Type[:6] == "input_" {
			n := g.Nodes[i]
			inputNode = &n
			break
		}
	}
	if inputNode == nil {
		return nil, fmt.Errorf("flowengine: flow %s has no input node", flow.ID)
	}

	// Inject the source ID into the input node's config copy.
	inputConfig := copyData(inputNode.Data)
	inputConfig["source_id"] = sourceID

	visited := make(map[string]bool)

	// 3. Walk the graph from the input node following edges.
	// inStack tracks nodes on the current DFS path for cycle detection at
	// execution time (catches cycles that ValidateFlow did not see, e.g. when
	// a flow is loaded directly from DB without being validated first).
	inStack := make(map[string]bool)

	var walk func(nodeID string, dependsOn []string) error
	walk = func(nodeID string, dependsOn []string) error {
		// Context cancellation check. Guard against nil ctx (used in some tests
		// via the //nolint:staticcheck exemption on the caller side).
		if ctx != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}

		// Cycle guard: if this node is already on the current DFS path we have
		// a back-edge.  Return an error rather than silently truncating the graph.
		if inStack[nodeID] {
			return fmt.Errorf("%w: back-edge detected at node %s during execution", errCyclicDAG, nodeID)
		}

		// Visit-once guard for diamond/converging paths (not a cycle).
		if visited[nodeID] {
			return nil
		}

		visited[nodeID] = true
		inStack[nodeID] = true
		defer func() { inStack[nodeID] = false }()

		node, ok := nodeByID[nodeID]
		if !ok {
			return fmt.Errorf("flowengine: node %s not found in graph", nodeID)
		}

		cfg := copyData(node.Data)
		if nodeID == inputNode.ID {
			cfg = inputConfig
		}

		// Error-edge routing: if the node signals a simulated failure (used in
		// tests and by error-injection tooling), follow "error" handle edges
		// instead of the normal successor edges.  In a real run the caller marks
		// the task as failed; the engine re-expands with simulate_error=true.
		nodeSimulatesError := func() bool {
			switch v := cfg["simulate_error"].(type) {
			case bool:
				return v
			case string:
				return v == "true"
			}
			return false
		}()

		// 4. For condition nodes: evaluate and follow only the matching branch.
		if node.Type == "condition" {
			result := evaluateCondition(cfg, analysisSummary)
			handle := "false"
			if result {
				handle = "true"
			}
			if nodeSimulatesError {
				handle = "error"
			}

			// Record the condition node itself as a step.
			steps = append(steps, TaskStep{
				NodeID:    nodeID,
				NodeType:  node.Type,
				Config:    cfg,
				DependsOn: dependsOn,
			})

			for _, edge := range edgesFrom[nodeID] {
				if edge.SourceHandle == handle || edge.SourceHandle == "" {
					if err := walk(edge.Target, []string{nodeID}); err != nil {
						return err
					}
				}
			}
			return nil
		}

		// 5. For all other node types: record as a step.
		steps = append(steps, TaskStep{
			NodeID:    nodeID,
			NodeType:  node.Type,
			Config:    cfg,
			DependsOn: dependsOn,
		})

		// 6. Continue walking edges. When the node signals failure, follow only
		// "error" handle edges; otherwise follow all non-"error" outgoing edges.
		for _, edge := range edgesFrom[nodeID] {
			if nodeSimulatesError {
				if edge.SourceHandle == "error" {
					if err := walk(edge.Target, []string{nodeID}); err != nil {
						return err
					}
				}
			} else {
				if edge.SourceHandle != "error" {
					if err := walk(edge.Target, []string{nodeID}); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}

	if err := walk(inputNode.ID, nil); err != nil {
		return nil, err
	}

	return steps, nil
}

// evaluateCondition inspects a condition node's config map and returns the
// boolean result.  Supported keys:
//   - "expression": a string that is tested for truthiness ("true", "1", non-empty)
//   - "operator" + "left" + "right": simple string or numeric comparison
//     ("eq", "neq", "gt", "lt", "gte", "lte")
//   - "field" + "operator" + "value": compare a field from source analysis
//     data (file_size, resolution, codec, total_frames, …) against a literal
//
// analysisSummary is the parsed summary JSON from the source analysis result.
// It may be nil/empty when no analysis data is available.
func evaluateCondition(cfg map[string]any, analysisSummary map[string]any) bool {
	// Field-based condition: look up a source analysis field.
	if field, ok := cfg["field"].(string); ok && field != "" {
		op, _ := cfg["operator"].(string)
		wantStr, _ := cfg["value"].(string)
		if analysisSummary != nil {
			if rawVal, exists := analysisSummary[field]; exists {
				got := fmt.Sprintf("%v", rawVal)
				return compareStrings(op, got, wantStr)
			}
		}
		// Field not in analysis data — treat as false.
		return false
	}

	// Simple expression truthiness check.
	if expr, ok := cfg["expression"]; ok {
		switch v := expr.(type) {
		case bool:
			return v
		case string:
			return v != "" && v != "false" && v != "0"
		case float64:
			return v != 0
		}
	}

	// Operator-based comparison of literal left/right strings.
	// Only attempt this when at least one of the operator keys is present.
	_, hasOp := cfg["operator"]
	_, hasLeft := cfg["left"]
	_, hasRight := cfg["right"]
	if hasOp || hasLeft || hasRight {
		op, _ := cfg["operator"].(string)
		left, _ := cfg["left"].(string)
		right, _ := cfg["right"].(string)
		return compareStrings(op, left, right)
	}

	// Default: treat non-empty config as true (legacy behaviour for
	// condition nodes that carry only metadata fields like "label").
	return len(cfg) > 0
}

// compareStrings compares two string values using the given operator.
// Numeric comparison is attempted first; falls back to lexicographic order.
func compareStrings(op, left, right string) bool {
	// Try numeric comparison.
	lf, lErr := strconv.ParseFloat(strings.TrimSpace(left), 64)
	rf, rErr := strconv.ParseFloat(strings.TrimSpace(right), 64)
	if lErr == nil && rErr == nil {
		switch op {
		case "eq":
			return lf == rf
		case "neq":
			return lf != rf
		case "gt":
			return lf > rf
		case "lt":
			return lf < rf
		case "gte":
			return lf >= rf
		case "lte":
			return lf <= rf
		}
	}
	// String comparison fallback.
	switch op {
	case "eq":
		return left == right
	case "neq":
		return left != right
	case "gt":
		return left > right
	case "lt":
		return left < right
	case "gte":
		return left >= right
	case "lte":
		return left <= right
	}
	// No recognized operator: treat as truthy if left is non-empty.
	// This preserves the legacy behaviour where a condition node with any
	// non-empty config (e.g. just a label) evaluated to true.
	return left != "" || right != ""
}

// copyData returns a shallow copy of a node data map to prevent mutation of
// the original parsed graph.
func copyData(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
