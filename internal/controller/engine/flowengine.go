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

// ExecuteFlow parses the flow graph stored in flow, finds the input node,
// walks the edges in order, evaluates condition nodes, and returns the
// resulting ordered list of TaskSteps.  The sourceID is passed through to
// any input node's config so callers can map it back to a source record.
//
// Condition nodes are evaluated against live source analysis data when
// available, so branches are resolved at expansion time rather than deferred.
func (fe *FlowEngine) ExecuteFlow(ctx context.Context, flow *db.Flow, sourceID string) ([]TaskStep, error) {
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

	var steps []TaskStep
	visited := make(map[string]bool)

	// 3. Walk the graph from the input node following edges.
	var walk func(nodeID string, dependsOn []string) error
	walk = func(nodeID string, dependsOn []string) error {
		if visited[nodeID] {
			return nil
		}
		visited[nodeID] = true

		node, ok := nodeByID[nodeID]
		if !ok {
			return fmt.Errorf("flowengine: node %s not found in graph", nodeID)
		}

		cfg := copyData(node.Data)
		if nodeID == inputNode.ID {
			cfg = inputConfig
		}

		// 4. For condition nodes: evaluate and follow only the matching branch.
		if node.Type == "condition" {
			result := evaluateCondition(cfg, analysisSummary)
			handle := "false"
			if result {
				handle = "true"
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

		// 5. For webhook nodes: record as a step (webhook delivery is queued
		//    by the caller after task execution).
		step := TaskStep{
			NodeID:    nodeID,
			NodeType:  node.Type,
			Config:    cfg,
			DependsOn: dependsOn,
		}
		steps = append(steps, step)

		// 6. Continue walking all outgoing edges from this node.
		for _, edge := range edgesFrom[nodeID] {
			if err := walk(edge.Target, []string{nodeID}); err != nil {
				return err
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
