//go:build rig

package rigtest

import (
	"fmt"
	"time"
)

// WaitForTogether polls until one snapshot of the palette holds every text. WaitFor reads one snapshot per call, so a
// state that is only true together, such as an editor's text and the status shown next to it, needs this.
func (a *App) WaitForTogether(timeout time.Duration, texts ...string) error {
	deadline := time.Now().Add(timeout)
	for {
		tree, err := a.Tree()
		if err != nil {
			return err
		}
		if treeHoldsEveryText(tree, texts) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("no palette snapshot holds all of %q within %s; labels seen: %s", texts, timeout, summarize(treeLabels(tree)))
		}
		time.Sleep(pollInterval)
	}
}

func treeHoldsEveryText(tree Tree, texts []string) bool {
	for _, text := range texts {
		if !treeHasText(tree, text) {
			return false
		}
	}
	return true
}

func treeHasText(tree Tree, text string) bool {
	for _, node := range tree.Nodes {
		if nodeHasText(node, text) {
			return true
		}
	}
	return false
}

func treeLabels(tree Tree) []string {
	var labels []string
	for _, node := range tree.Nodes {
		if node.Label != "" {
			labels = append(labels, node.Label)
		}
	}
	return labels
}
