package device

import (
	"cmp"
	"path/filepath"
	"slices"
)

func comparePaths(a, b string) int {
	an, aok := hidrawNumber(filepath.Base(a))
	bn, bok := hidrawNumber(filepath.Base(b))
	if aok && bok {
		if order := cmp.Compare(an, bn); order != 0 {
			return order
		}
	}
	return cmp.Compare(a, b)
}

func groupNodes(nodes []Node) ([]Group, []Diagnostic) {
	groups := []Group{}
	findings := []Diagnostic{}
	byParent := map[string][]Node{}
	for _, n := range nodes {
		if n.Admission == "admitted" {
			byParent[*n.USBParent] = append(byParent[*n.USBParent], n)
		}
	}
	for parent, members := range byParent {
		g := Group{USBParent: parent, HIDPaths: []string{}}

		for _, n := range members {
			g.HIDPaths = append(g.HIDPaths, n.Path)

		}
		g.Complete = len(members) == 1
		slices.SortFunc(g.HIDPaths, comparePaths)
		groups = append(groups, g)
		if !g.Complete {
			findings = append(findings, diagnostic(ERR_DEVICE_AMBIGUOUS, "grouping", &parent))
		}
	}
	slices.SortFunc(groups, func(a, b Group) int { return cmp.Compare(a.USBParent, b.USBParent) })
	return groups, findings
}

func sortResult(r *Result) {
	slices.SortFunc(r.Nodes, func(a, b Node) int { return comparePaths(a.Path, b.Path) })
	slices.SortStableFunc(r.Diagnostics, func(a, b Diagnostic) int {
		at, bt := "", ""
		if a.Target != nil {
			at = *a.Target
		}
		if b.Target != nil {
			bt = *b.Target
		}
		if order := cmp.Compare(at, bt); order != 0 {
			return order
		}
		return cmp.Compare(a.Code, b.Code)
	})
}
