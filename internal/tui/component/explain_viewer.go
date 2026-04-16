package component

import (
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/rs/zerolog/log"
)

const (
	ExplainViewerId = "ExplainViewer"
)

var explainViewerCounter int32

// ExplainViewer renders an EXPLAIN plan as a navigable TreeView overlay.
type ExplainViewer struct {
	*core.BaseElement
	*core.Flex

	tree        *core.TreeView
	analyzeMode bool
	explainFunc func(analyze bool) (string, error)
	doneFunc    func()
}

func NewExplainViewer() *ExplainViewer {
	n := atomic.AddInt32(&explainViewerCounter, 1)
	id := tview.Identifier(fmt.Sprintf("%s-%d", ExplainViewerId, n))
	e := &ExplainViewer{
		BaseElement: core.NewBaseElement(),
		Flex:        core.NewFlex(),
		tree:        core.NewTreeView(),
		analyzeMode: false,
	}

	e.SetIdentifier(id)
	e.SetAfterInitFunc(e.init)

	return e
}

func (e *ExplainViewer) init() error {
	e.setLayout()
	e.setStyle()
	e.setKeybindings()
	e.handleEvents()
	return nil
}

func (e *ExplainViewer) handleEvents() {
	go e.HandleEvents(e.GetIdentifier(), func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.StyleChanged:
			e.setStyle()
		}
	})
}

func (e *ExplainViewer) setLayout() {
	e.Flex.SetBorder(true)
	e.Flex.SetTitleAlign(tview.AlignCenter)
	e.Flex.SetDirection(tview.FlexRow)
	e.Flex.AddItem(e.tree, 0, 1, true)
	e.updateTitle()
}

func (e *ExplainViewer) updateTitle() {
	if e.analyzeMode {
		e.Flex.SetTitle(" EXPLAIN ANALYZE  [dim](t) plan only[-:-:-] ")
	} else {
		e.Flex.SetTitle(" EXPLAIN Plan  [dim](t) analyze — executes query[-:-:-] ")
	}
}

func (e *ExplainViewer) setStyle() {
	e.Flex.SetStyle(e.App.GetStyles())
	e.tree.SetStyle(e.App.GetStyles())
}

func (e *ExplainViewer) setKeybindings() {
	k := e.App.GetKeys()
	e.tree.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case k.Contains(k.Common.Close, event.Name()):
			if e.doneFunc != nil {
				e.doneFunc()
			}
			return nil
		case k.Contains(k.ExplainViewer.ToggleMode, event.Name()):
			e.toggleMode()
			return nil
		}
		return event
	})
}

func (e *ExplainViewer) toggleMode() {
	if e.explainFunc == nil {
		return
	}
	e.analyzeMode = !e.analyzeMode
	result, err := e.explainFunc(e.analyzeMode)
	if err != nil {
		log.Error().Err(err).Bool("analyze", e.analyzeMode).Msg("Failed to re-run explain")
		e.analyzeMode = !e.analyzeMode // revert
		return
	}
	e.Render(result)
}

func (e *ExplainViewer) SetDoneFunc(fn func()) {
	e.doneFunc = fn
}

func (e *ExplainViewer) SetExplainFunc(fn func(analyze bool) (string, error)) {
	e.explainFunc = fn
}

// Render builds the tree from result, which is either a Postgres JSON plan or
// plain-text SQLite EXPLAIN QUERY PLAN output.
func (e *ExplainViewer) Render(result string) {
	e.updateTitle()

	root := tview.NewTreeNode("EXPLAIN")
	root.SetExpanded(true)

	trimmed := strings.TrimSpace(result)
	if strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "{") {
		plan, err := database.ParsePostgresPlan(trimmed)
		if err != nil {
			log.Error().Err(err).Msg("Failed to parse EXPLAIN JSON plan")
			root.SetText("Parse error: " + err.Error())
		} else {
			buildTreeNode(root, plan, e.analyzeMode)
		}
	} else {
		// SQLite plain text: one line per node
		for line := range strings.SplitSeq(trimmed, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				root.AddChild(tview.NewTreeNode(line))
			}
		}
	}

	e.tree.SetRoot(root)
	e.tree.SetCurrentNode(root)
}

func buildTreeNode(parent *tview.TreeNode, node *database.PlanNode, analyze bool) {
	label := node.NodeType
	if node.RelationName != "" {
		label += " on " + node.RelationName
	}
	if analyze {
		label += fmt.Sprintf("  cost=%.2f..%.2f  rows=%d/%d  loops=%d",
			node.StartupCost, node.TotalCost, node.ActualRows, node.PlanRows, node.ActualLoops)
	} else {
		label += fmt.Sprintf("  cost=%.2f..%.2f  rows=%d",
			node.StartupCost, node.TotalCost, node.PlanRows)
	}

	treeNode := tview.NewTreeNode(label)
	treeNode.SetExpanded(true)

	// Show timing info only in analyze mode (plan-only has no execution stats)
	if analyze && (node.PlanningMs > 0 || node.ExecutionMs > 0) {
		treeNode.AddChild(tview.NewTreeNode(
			fmt.Sprintf("Planning: %.3f ms  Execution: %.3f ms", node.PlanningMs, node.ExecutionMs)))
	}

	for _, child := range node.Children {
		buildTreeNode(treeNode, child, analyze)
	}

	parent.AddChild(treeNode)
}
