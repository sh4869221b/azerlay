package device

import (
	"path/filepath"
	"testing"
)

func TestGroupTopology(t *testing.T) {
	t.Parallel()
	c := fixtureCollector(t)
	nodes := []Node{}
	for i := 1; i <= 3; i++ {
		path := fixtureNode(t, c, "unit", i, i)
		m := c.metadata(path, filepath.Base(path))
		admit(&m)
		nodes = append(nodes, m.node)
	}
	groups, findings := groupNodes(nodes)
	if len(groups) != 1 || !groups[0].Complete || len(groups[0].EventPaths) != 3 || len(findings) != 0 || len(nodes[0].Roles) != 3 {
		t.Fatalf("complete group: %+v %+v", groups, findings)
	}
	path := fixtureNode(t, c, "second-unit", 10, 1)
	m := c.metadata(path, filepath.Base(path))
	admit(&m)
	nodes = append(nodes, m.node)
	groups, findings = groupNodes(nodes)
	if len(groups) != 2 || len(findings) != 1 {
		t.Fatalf("same-serial parents merged: %+v", groups)
	}
	nodes[1].Admission = "unsupported"
	groups, findings = groupNodes(nodes)
	if len(groups) != 2 || len(findings) != 2 {
		t.Fatalf("excluded sibling not reflected: %+v", groups)
	}
	for _, g := range groups {
		if g.Complete {
			t.Fatal("excluded node completed group")
		}
	}
}
