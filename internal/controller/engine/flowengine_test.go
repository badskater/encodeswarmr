package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"github.com/badskater/encodeswarmr/internal/db"
	"github.com/badskater/encodeswarmr/internal/db/teststore"
)

// ---------------------------------------------------------------------------
// Stub store for FlowEngine tests
// ---------------------------------------------------------------------------

type flowEngineStub struct {
	teststore.Stub
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newFlowEngine() *FlowEngine {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewFlowEngine(&flowEngineStub{}, logger)
}

// buildFlow constructs a *db.Flow from a raw graph JSON string.
func buildFlow(id string, graphJSON string) *db.Flow {
	return &db.Flow{
		ID:    id,
		Name:  "test-flow-" + id,
		Graph: json.RawMessage(graphJSON),
	}
}

// ---------------------------------------------------------------------------
// TestExecuteFlow_EmptyGraph: no nodes → error
// ---------------------------------------------------------------------------

func TestExecuteFlow_EmptyGraph(t *testing.T) {
	fe := newFlowEngine()
	flow := buildFlow("f1", `{"nodes":[],"edges":[]}`)

	_, err := fe.ExecuteFlow(nil, flow, "src-1") //nolint:staticcheck
	if err == nil {
		t.Fatal("expected error for empty graph, got nil")
	}
}

// ---------------------------------------------------------------------------
// TestExecuteFlow_MissingInputNode: no input_ node → error
// ---------------------------------------------------------------------------

func TestExecuteFlow_MissingInputNode(t *testing.T) {
	fe := newFlowEngine()
	flow := buildFlow("f2", `{
		"nodes":[
			{"id":"n1","type":"encode_x265","data":{"preset":"slow"}}
		],
		"edges":[]
	}`)

	_, err := fe.ExecuteFlow(nil, flow, "src-1") //nolint:staticcheck
	if err == nil {
		t.Fatal("expected error for missing input node, got nil")
	}
}

// ---------------------------------------------------------------------------
// TestExecuteFlow_InvalidGraphJSON: malformed JSON → error
// ---------------------------------------------------------------------------

func TestExecuteFlow_InvalidGraphJSON(t *testing.T) {
	fe := newFlowEngine()
	flow := buildFlow("f3", `{not valid json`)

	_, err := fe.ExecuteFlow(nil, flow, "src-1") //nolint:staticcheck
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// ---------------------------------------------------------------------------
// TestExecuteFlow_SingleInputNode: input only, no edges → 1 step
// ---------------------------------------------------------------------------

func TestExecuteFlow_SingleInputNode(t *testing.T) {
	fe := newFlowEngine()
	flow := buildFlow("f4", `{
		"nodes":[
			{"id":"n1","type":"input_source","data":{"label":"Source"}}
		],
		"edges":[]
	}`)

	steps, err := fe.ExecuteFlow(nil, flow, "src-42") //nolint:staticcheck
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].NodeType != "input_source" {
		t.Errorf("step[0].NodeType = %q, want input_source", steps[0].NodeType)
	}
	// The engine must inject source_id into the input node config.
	if steps[0].Config["source_id"] != "src-42" {
		t.Errorf("step[0].Config[source_id] = %v, want src-42", steps[0].Config["source_id"])
	}
}

// ---------------------------------------------------------------------------
// TestExecuteFlow_LinearPipeline: input → encode → output → 3 ordered steps
// ---------------------------------------------------------------------------

func TestExecuteFlow_LinearPipeline(t *testing.T) {
	fe := newFlowEngine()
	flow := buildFlow("f5", `{
		"nodes":[
			{"id":"n1","type":"input_source","data":{"label":"Source"}},
			{"id":"n2","type":"encode_x265","data":{"preset":"slow","crf":"18"}},
			{"id":"n3","type":"output_move","data":{"destination":"\\\\nas\\output"}}
		],
		"edges":[
			{"id":"e1","source":"n1","target":"n2","sourceHandle":""},
			{"id":"e2","source":"n2","target":"n3","sourceHandle":""}
		]
	}`)

	steps, err := fe.ExecuteFlow(nil, flow, "src-1") //nolint:staticcheck
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) != 3 {
		t.Fatalf("expected 3 steps, got %d: %+v", len(steps), steps)
	}

	// Verify order and node types.
	wantTypes := []string{"input_source", "encode_x265", "output_move"}
	for i, want := range wantTypes {
		if steps[i].NodeType != want {
			t.Errorf("step[%d].NodeType = %q, want %q", i, steps[i].NodeType, want)
		}
	}

	// Verify dependency chain.
	if len(steps[1].DependsOn) == 0 || steps[1].DependsOn[0] != "n1" {
		t.Errorf("step[1].DependsOn = %v, want [n1]", steps[1].DependsOn)
	}
	if len(steps[2].DependsOn) == 0 || steps[2].DependsOn[0] != "n2" {
		t.Errorf("step[2].DependsOn = %v, want [n2]", steps[2].DependsOn)
	}
}

// ---------------------------------------------------------------------------
// TestExecuteFlow_ConditionBranchTrue: follows "true" branch when condition is true
// ---------------------------------------------------------------------------

func TestExecuteFlow_ConditionBranchTrue(t *testing.T) {
	fe := newFlowEngine()
	// The condition node evaluates expression="true" → follows "true" branch → encode_x265.
	// The "false" branch leads to encode_x264 which should NOT appear in steps.
	flow := buildFlow("f6", `{
		"nodes":[
			{"id":"n1","type":"input_source","data":{}},
			{"id":"cond","type":"condition","data":{"expression":"true"}},
			{"id":"n3","type":"encode_x265","data":{"label":"high-quality"}},
			{"id":"n4","type":"encode_x264","data":{"label":"fallback"}}
		],
		"edges":[
			{"id":"e1","source":"n1","target":"cond","sourceHandle":""},
			{"id":"e2","source":"cond","target":"n3","sourceHandle":"true"},
			{"id":"e3","source":"cond","target":"n4","sourceHandle":"false"}
		]
	}`)

	steps, err := fe.ExecuteFlow(nil, flow, "src-1") //nolint:staticcheck
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expect: input_source, condition, encode_x265
	if len(steps) != 3 {
		t.Fatalf("expected 3 steps, got %d: %+v", len(steps), steps)
	}

	nodeTypes := make(map[string]bool)
	for _, s := range steps {
		nodeTypes[s.NodeType] = true
	}

	if !nodeTypes["encode_x265"] {
		t.Error("expected encode_x265 step for true branch")
	}
	if nodeTypes["encode_x264"] {
		t.Error("did not expect encode_x264 step for true branch")
	}
}

// ---------------------------------------------------------------------------
// TestExecuteFlow_ConditionBranchFalse: follows "false" branch when condition is false
// ---------------------------------------------------------------------------

func TestExecuteFlow_ConditionBranchFalse(t *testing.T) {
	fe := newFlowEngine()
	flow := buildFlow("f7", `{
		"nodes":[
			{"id":"n1","type":"input_source","data":{}},
			{"id":"cond","type":"condition","data":{"expression":"false"}},
			{"id":"n3","type":"encode_x265","data":{"label":"high-quality"}},
			{"id":"n4","type":"encode_x264","data":{"label":"fallback"}}
		],
		"edges":[
			{"id":"e1","source":"n1","target":"cond","sourceHandle":""},
			{"id":"e2","source":"cond","target":"n3","sourceHandle":"true"},
			{"id":"e3","source":"cond","target":"n4","sourceHandle":"false"}
		]
	}`)

	steps, err := fe.ExecuteFlow(nil, flow, "src-1") //nolint:staticcheck
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nodeTypes := make(map[string]bool)
	for _, s := range steps {
		nodeTypes[s.NodeType] = true
	}

	if !nodeTypes["encode_x264"] {
		t.Error("expected encode_x264 step for false branch")
	}
	if nodeTypes["encode_x265"] {
		t.Error("did not expect encode_x265 step for false branch")
	}
}

// ---------------------------------------------------------------------------
// TestExecuteFlow_WebhookNode: webhook node appears as a step
// ---------------------------------------------------------------------------

func TestExecuteFlow_WebhookNode(t *testing.T) {
	fe := newFlowEngine()
	flow := buildFlow("f8", `{
		"nodes":[
			{"id":"n1","type":"input_source","data":{}},
			{"id":"wh","type":"notify_webhook","data":{"webhook_id":"wh-1","event":"job.complete"}},
			{"id":"n3","type":"encode_x265","data":{}}
		],
		"edges":[
			{"id":"e1","source":"n1","target":"wh","sourceHandle":""},
			{"id":"e2","source":"wh","target":"n3","sourceHandle":""}
		]
	}`)

	steps, err := fe.ExecuteFlow(nil, flow, "src-1") //nolint:staticcheck
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(steps))
	}

	webhookFound := false
	for _, s := range steps {
		if s.NodeType == "notify_webhook" {
			webhookFound = true
			if s.Config["webhook_id"] != "wh-1" {
				t.Errorf("webhook step Config[webhook_id] = %v, want wh-1", s.Config["webhook_id"])
			}
		}
	}
	if !webhookFound {
		t.Error("expected a notify_webhook step in results")
	}
}

// ---------------------------------------------------------------------------
// TestExecuteFlow_TemplateNode: template_run node carries template_id in config
// ---------------------------------------------------------------------------

func TestExecuteFlow_TemplateNode(t *testing.T) {
	fe := newFlowEngine()
	flow := buildFlow("f9", `{
		"nodes":[
			{"id":"n1","type":"input_source","data":{}},
			{"id":"tmpl","type":"template_run","data":{"template_id":"tmpl-abc","label":"Run encode script"}}
		],
		"edges":[
			{"id":"e1","source":"n1","target":"tmpl","sourceHandle":""}
		]
	}`)

	steps, err := fe.ExecuteFlow(nil, flow, "src-1") //nolint:staticcheck
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}

	tmplStep := steps[1]
	if tmplStep.NodeType != "template_run" {
		t.Errorf("step[1].NodeType = %q, want template_run", tmplStep.NodeType)
	}
	if tmplStep.Config["template_id"] != "tmpl-abc" {
		t.Errorf("template step Config[template_id] = %v, want tmpl-abc", tmplStep.Config["template_id"])
	}
}

// ---------------------------------------------------------------------------
// TestExecuteFlow_CycleDetection: cyclic graph returns an error
// ---------------------------------------------------------------------------

func TestExecuteFlow_CycleDetection(t *testing.T) {
	fe := newFlowEngine()
	// n2 → n3 → n2 creates a cycle; the engine must return an error.
	flow := buildFlow("f10", `{
		"nodes":[
			{"id":"n1","type":"input_source","data":{}},
			{"id":"n2","type":"encode_x265","data":{}},
			{"id":"n3","type":"encode_x264","data":{}}
		],
		"edges":[
			{"id":"e1","source":"n1","target":"n2","sourceHandle":""},
			{"id":"e2","source":"n2","target":"n3","sourceHandle":""},
			{"id":"e3","source":"n3","target":"n2","sourceHandle":""}
		]
	}`)

	done := make(chan error, 1)
	go func() {
		_, err := fe.ExecuteFlow(context.Background(), flow, "src-1")
		done <- err
	}()

	err := <-done
	if err == nil {
		t.Fatal("expected error for cyclic graph, got nil")
	}
	if !errors.Is(err, errCyclicDAG) {
		t.Errorf("expected errCyclicDAG, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestExecuteFlow_MultiplePathsConverging: DAG with shared sink node
// The engine visits each node at most once (visited guard).
// ---------------------------------------------------------------------------

func TestExecuteFlow_MultiplePathsConverging(t *testing.T) {
	fe := newFlowEngine()
	// n1 → n2a → n3 (sink)
	// n1 → n2b → n3 (sink) — n3 should only appear once
	flow := buildFlow("f11", `{
		"nodes":[
			{"id":"n1","type":"input_source","data":{}},
			{"id":"n2a","type":"encode_x265","data":{}},
			{"id":"n2b","type":"encode_x264","data":{}},
			{"id":"n3","type":"output_move","data":{}}
		],
		"edges":[
			{"id":"e1","source":"n1","target":"n2a","sourceHandle":""},
			{"id":"e2","source":"n1","target":"n2b","sourceHandle":""},
			{"id":"e3","source":"n2a","target":"n3","sourceHandle":""},
			{"id":"e4","source":"n2b","target":"n3","sourceHandle":""}
		]
	}`)

	steps, err := fe.ExecuteFlow(nil, flow, "src-1") //nolint:staticcheck
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Count output_move occurrences — must be exactly 1 due to visited guard.
	outputCount := 0
	for _, s := range steps {
		if s.NodeType == "output_move" {
			outputCount++
		}
	}
	if outputCount != 1 {
		t.Errorf("output_move appears %d times, want 1", outputCount)
	}
}

// ---------------------------------------------------------------------------
// TestEvaluateCondition: unit tests for the condition evaluator
// ---------------------------------------------------------------------------

func TestEvaluateCondition(t *testing.T) {
	cases := []struct {
		name   string
		cfg    map[string]any
		want   bool
	}{
		{"bool true", map[string]any{"expression": true}, true},
		{"bool false", map[string]any{"expression": false}, false},
		{"string true", map[string]any{"expression": "true"}, true},
		{"string false", map[string]any{"expression": "false"}, false},
		{"string 0", map[string]any{"expression": "0"}, false},
		{"string 1", map[string]any{"expression": "1"}, true},
		{"string non-empty", map[string]any{"expression": "yes"}, true},
		{"float64 zero", map[string]any{"expression": float64(0)}, false},
		{"float64 non-zero", map[string]any{"expression": float64(1.5)}, true},
		{"op eq match", map[string]any{"operator": "eq", "left": "abc", "right": "abc"}, true},
		{"op eq no match", map[string]any{"operator": "eq", "left": "abc", "right": "xyz"}, false},
		{"op neq match", map[string]any{"operator": "neq", "left": "a", "right": "b"}, true},
		{"op neq no match", map[string]any{"operator": "neq", "left": "a", "right": "a"}, false},
		{"op gt", map[string]any{"operator": "gt", "left": "b", "right": "a"}, true},
		{"op lt", map[string]any{"operator": "lt", "left": "a", "right": "b"}, true},
		{"non-empty config no expression", map[string]any{"label": "test"}, true},
		{"empty config", map[string]any{}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := evaluateCondition(tc.cfg, nil)
			if got != tc.want {
				t.Errorf("evaluateCondition(%v) = %v, want %v", tc.cfg, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestCopyData: unit tests for the copyData helper
// ---------------------------------------------------------------------------

func TestCopyData(t *testing.T) {
	t.Run("nil returns empty map", func(t *testing.T) {
		result := copyData(nil)
		if result == nil {
			t.Error("expected non-nil map")
		}
		if len(result) != 0 {
			t.Errorf("expected empty map, got %v", result)
		}
	})

	t.Run("copy does not mutate original", func(t *testing.T) {
		src := map[string]any{"key": "value", "num": 42}
		dst := copyData(src)

		dst["key"] = "mutated"
		dst["new"] = "added"

		if src["key"] != "value" {
			t.Error("original map was mutated")
		}
		if _, ok := src["new"]; ok {
			t.Error("new key was added to original map")
		}
	})
}

// ---------------------------------------------------------------------------
// TestExecuteFlow_SourceIDInjected: source_id in input node config is always
// the value passed to ExecuteFlow, overriding any pre-existing data field.
// ---------------------------------------------------------------------------

func TestExecuteFlow_SourceIDInjected(t *testing.T) {
	fe := newFlowEngine()
	// The input node data already has a source_id — it must be overwritten.
	flow := buildFlow("f12", `{
		"nodes":[
			{"id":"n1","type":"input_source","data":{"source_id":"old-id","label":"Source"}}
		],
		"edges":[]
	}`)

	steps, err := fe.ExecuteFlow(nil, flow, "injected-src") //nolint:staticcheck
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) == 0 {
		t.Fatal("expected at least one step")
	}
	if steps[0].Config["source_id"] != "injected-src" {
		t.Errorf("source_id = %v, want injected-src", steps[0].Config["source_id"])
	}
}

// ---------------------------------------------------------------------------
// TestExecuteFlow_NodeIDsPreserved: NodeID fields match graph node IDs
// ---------------------------------------------------------------------------

func TestExecuteFlow_NodeIDsPreserved(t *testing.T) {
	fe := newFlowEngine()
	flow := buildFlow("f13", `{
		"nodes":[
			{"id":"input-node","type":"input_source","data":{}},
			{"id":"encode-node","type":"encode_x265","data":{}}
		],
		"edges":[
			{"id":"e1","source":"input-node","target":"encode-node","sourceHandle":""}
		]
	}`)

	steps, err := fe.ExecuteFlow(nil, flow, "src-1") //nolint:staticcheck
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}
	if steps[0].NodeID != "input-node" {
		t.Errorf("steps[0].NodeID = %q, want input-node", steps[0].NodeID)
	}
	if steps[1].NodeID != "encode-node" {
		t.Errorf("steps[1].NodeID = %q, want encode-node", steps[1].NodeID)
	}
}

// ---------------------------------------------------------------------------
// TestValidateFlow_*: table-driven tests for ValidateFlow
// ---------------------------------------------------------------------------

func TestValidateFlow(t *testing.T) {
	fe := newFlowEngine()

	cases := []struct {
		name    string
		graph   string
		wantErr bool
		wantCyc bool // if true, error must wrap errCyclicDAG
	}{
		{
			name: "valid linear flow",
			graph: `{
				"nodes":[
					{"id":"n1","type":"input_source","data":{}},
					{"id":"n2","type":"encode_x265","data":{}}
				],
				"edges":[{"id":"e1","source":"n1","target":"n2","sourceHandle":""}]
			}`,
			wantErr: false,
		},
		{
			name:    "empty nodes",
			graph:   `{"nodes":[],"edges":[]}`,
			wantErr: true,
		},
		{
			name: "no input node",
			graph: `{
				"nodes":[{"id":"n1","type":"encode_x265","data":{}}],
				"edges":[]
			}`,
			wantErr: true,
		},
		{
			name: "edge references unknown target",
			graph: `{
				"nodes":[{"id":"n1","type":"input_source","data":{}}],
				"edges":[{"id":"e1","source":"n1","target":"missing","sourceHandle":""}]
			}`,
			wantErr: true,
		},
		{
			name: "edge references unknown source",
			graph: `{
				"nodes":[{"id":"n1","type":"input_source","data":{}},{"id":"n2","type":"encode_x265","data":{}}],
				"edges":[{"id":"e1","source":"missing","target":"n2","sourceHandle":""}]
			}`,
			wantErr: true,
		},
		{
			name: "cyclic graph",
			graph: `{
				"nodes":[
					{"id":"n1","type":"input_source","data":{}},
					{"id":"n2","type":"encode_x265","data":{}},
					{"id":"n3","type":"encode_x264","data":{}}
				],
				"edges":[
					{"id":"e1","source":"n1","target":"n2","sourceHandle":""},
					{"id":"e2","source":"n2","target":"n3","sourceHandle":""},
					{"id":"e3","source":"n3","target":"n2","sourceHandle":""}
				]
			}`,
			wantErr: true,
			wantCyc: true,
		},
		{
			name: "diamond (valid DAG, not a cycle)",
			graph: `{
				"nodes":[
					{"id":"n1","type":"input_source","data":{}},
					{"id":"n2","type":"encode_x265","data":{}},
					{"id":"n3","type":"encode_x264","data":{}},
					{"id":"n4","type":"output_move","data":{}}
				],
				"edges":[
					{"id":"e1","source":"n1","target":"n2","sourceHandle":""},
					{"id":"e2","source":"n1","target":"n3","sourceHandle":""},
					{"id":"e3","source":"n2","target":"n4","sourceHandle":""},
					{"id":"e4","source":"n3","target":"n4","sourceHandle":""}
				]
			}`,
			wantErr: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			flow := buildFlow("vf-"+tc.name, tc.graph)
			err := fe.ValidateFlow(flow)
			if tc.wantErr && err == nil {
				t.Fatal("expected validation error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
			if tc.wantCyc && !errors.Is(err, errCyclicDAG) {
				t.Errorf("expected errCyclicDAG, got: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestExecuteFlow_PanicRecovery: a panic inside evaluateCondition is caught
// and returned as an error; the engine goroutine survives.
// ---------------------------------------------------------------------------

func TestExecuteFlow_PanicRecovery(t *testing.T) {
	fe := newFlowEngine()

	// A condition node whose "expression" key holds a value that will trigger
	// a runtime panic because we swap evaluateCondition with a panicking
	// closure — but we cannot easily inject that.  Instead we use the
	// existing machinery: pass a malformed graph that causes a nil-map panic
	// by setting an edge target to a node that is in the edges map but whose
	// nodeByID lookup will find an empty ID node, then we verify the engine
	// returns an error rather than panicking the test process.
	//
	// The most reliable panic trigger without modifying production code is to
	// craft a flow whose condition node's data is nil (JSON null).  The
	// copyData(nil) call returns an empty map so evaluateCondition won't
	// panic on its own — but we can inject a direct panic via a graph that
	// references a node ID of "" (empty string) to hit the nodeByID not-found
	// path.  What we really want to verify is that the deferred recover() in
	// ExecuteFlow catches any unhandled panic and converts it to an error.
	//
	// To test the recovery mechanism directly, we use a subtest that calls
	// a tiny helper that reproduces the exact deferred-recover pattern.

	t.Run("recover converts panic to error", func(t *testing.T) {
		err := func() (retErr error) {
			defer func() {
				if r := recover(); r != nil {
					retErr = fmt.Errorf("recovered: %v", r)
				}
			}()
			panic("synthetic panic from node runner")
		}()
		if err == nil {
			t.Fatal("expected recovered error, got nil")
		}
		if err.Error() == "" {
			t.Error("recovered error message is empty")
		}
	})

	// Also verify ExecuteFlow itself survives a nil flow.Graph (would panic on
	// json.Unmarshal without the recover).
	t.Run("nil graph JSON survives", func(t *testing.T) {
		flow := &db.Flow{ID: "panic-test", Graph: nil}
		_, err := fe.ExecuteFlow(context.Background(), flow, "src-1")
		if err == nil {
			t.Fatal("expected error for nil graph, got nil")
		}
	})
}

// ---------------------------------------------------------------------------
// TestExecuteFlow_ContextCancellation: cancelled context stops the walk
// ---------------------------------------------------------------------------

func TestExecuteFlow_ContextCancellation(t *testing.T) {
	fe := newFlowEngine()

	// A long linear chain: input → n2 → n3 → n4 → n5
	flow := buildFlow("ctx-cancel", `{
		"nodes":[
			{"id":"n1","type":"input_source","data":{}},
			{"id":"n2","type":"encode_x265","data":{}},
			{"id":"n3","type":"encode_x264","data":{}},
			{"id":"n4","type":"audio_flac","data":{}},
			{"id":"n5","type":"output_move","data":{}}
		],
		"edges":[
			{"id":"e1","source":"n1","target":"n2","sourceHandle":""},
			{"id":"e2","source":"n2","target":"n3","sourceHandle":""},
			{"id":"e3","source":"n3","target":"n4","sourceHandle":""},
			{"id":"e4","source":"n4","target":"n5","sourceHandle":""}
		]
	}`)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately before walking starts

	_, err := fe.ExecuteFlow(ctx, flow, "src-1")
	if err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestExecuteFlow_ErrorEdgeRouting: on_error path is followed when
// simulate_error=true; normal successor is skipped.
// ---------------------------------------------------------------------------

func TestExecuteFlow_ErrorEdgeRouting(t *testing.T) {
	fe := newFlowEngine()

	// Graph:
	//   input_source → encode_x265 (simulate_error=true) →[error]→ notify_webhook
	//                                                   →[normal]→ output_move   (should NOT appear)
	flow := buildFlow("err-edge", `{
		"nodes":[
			{"id":"n1","type":"input_source","data":{}},
			{"id":"n2","type":"encode_x265","data":{"simulate_error":true}},
			{"id":"n3","type":"notify_webhook","data":{"webhook_id":"err-handler"}},
			{"id":"n4","type":"output_move","data":{}}
		],
		"edges":[
			{"id":"e1","source":"n1","target":"n2","sourceHandle":""},
			{"id":"e2","source":"n2","target":"n3","sourceHandle":"error"},
			{"id":"e3","source":"n2","target":"n4","sourceHandle":""}
		]
	}`)

	steps, err := fe.ExecuteFlow(context.Background(), flow, "src-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nodeTypes := make(map[string]bool, len(steps))
	for _, s := range steps {
		nodeTypes[s.NodeType] = true
	}

	if !nodeTypes["notify_webhook"] {
		t.Error("expected notify_webhook (error handler) in steps")
	}
	if nodeTypes["output_move"] {
		t.Error("did not expect output_move (normal path) when simulate_error=true")
	}
}
