package sql

import "encoding/json"

// PlanNode mirrors the Postgres JSON plan node structure.
type PlanNode struct {
	NodeType     string
	RelationName string
	StartupCost  float64
	TotalCost    float64
	PlanRows     int64
	ActualRows   int64
	ActualLoops  int64
	PlanningMs   float64
	ExecutionMs  float64
	Children     []*PlanNode
}

// rawPlanNode is used only for JSON unmarshalling.
// Postgres outputs Actual Rows / Actual Loops / Plan Rows as floats in JSON.
type rawPlanNode struct {
	NodeType     string         `json:"Node Type"`
	RelationName string         `json:"Relation Name"`
	StartupCost  float64        `json:"Startup Cost"`
	TotalCost    float64        `json:"Total Cost"`
	PlanRows     float64        `json:"Plan Rows"`
	ActualRows   float64        `json:"Actual Rows"`
	ActualLoops  float64        `json:"Actual Loops"`
	Plans        []*rawPlanNode `json:"Plans"`
}

type rawTopLevel struct {
	Plan        rawPlanNode `json:"Plan"`
	PlanningMs  float64     `json:"Planning Time"`
	ExecutionMs float64     `json:"Execution Time"`
}

func convertNode(r *rawPlanNode) *PlanNode {
	n := &PlanNode{
		NodeType:     r.NodeType,
		RelationName: r.RelationName,
		StartupCost:  r.StartupCost,
		TotalCost:    r.TotalCost,
		PlanRows:     int64(r.PlanRows),
		ActualRows:   int64(r.ActualRows),
		ActualLoops:  int64(r.ActualLoops),
	}
	for _, child := range r.Plans {
		n.Children = append(n.Children, convertNode(child))
	}
	return n
}

// ParsePostgresPlan parses the JSON string returned by
// EXPLAIN (ANALYZE, FORMAT JSON) into a PlanNode tree.
func ParsePostgresPlan(jsonStr string) (*PlanNode, error) {
	var top []rawTopLevel
	if err := json.Unmarshal([]byte(jsonStr), &top); err != nil {
		return nil, err
	}
	if len(top) == 0 {
		return &PlanNode{NodeType: "Empty"}, nil
	}
	root := convertNode(&top[0].Plan)
	root.PlanningMs = top[0].PlanningMs
	root.ExecutionMs = top[0].ExecutionMs
	return root, nil
}
