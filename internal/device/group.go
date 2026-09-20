package device

import (
	"cmp"
	"path/filepath"
	"slices"
)

func comparePaths(a, b string) int {
	an, aok := eventNumber(filepath.Base(a))
	bn, bok := eventNumber(filepath.Base(b))
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
		g := Group{USBParent: parent, EventPaths: []string{}}
		interfaces := map[uint8]bool{}
		for _, n := range members {
			g.EventPaths = append(g.EventPaths, n.Path)
			interfaces[n.Interface.Number] = true
		}
		g.Complete = interfaces[1] && interfaces[2] && interfaces[3]
		slices.SortFunc(g.EventPaths, comparePaths)
		groups = append(groups, g)
		if !g.Complete {
			findings = append(findings, diagnostic(WARN_DEVICE_INCOMPLETE, "grouping", &parent))
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
