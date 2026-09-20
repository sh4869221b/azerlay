package device

import (
	"path/filepath"
	"testing"
)

func TestMatchContract(t *testing.T) {
	for _, tc := range []struct {
		name      string
		change    func(*metadata)
		admission string
	}{
		{"valid", func(m *metadata) {}, "admitted"},
		{"extra capabilities", func(m *metadata) { m.node.Capabilities["key"] = append(m.node.Capabilities["key"], 705) }, "admitted"},
		{"missing serial", func(m *metadata) { m.node.Serial = nil }, "admitted"},
		{"different release", func(m *metadata) { m.node.USBID.Release = 0x112 }, "unsupported"},
		{"different vendor", func(m *metadata) { m.node.USBID.Vendor = 0x1234 }, "unsupported"},
		{"different product", func(m *metadata) { m.node.USBID.Product = 0x5678 }, "unsupported"},
		{"unknown interface", func(m *metadata) { m.node.Interface.Number = 4 }, "unsupported"},
		{"wrong tuple", func(m *metadata) { m.node.Interface.Protocol = 1 }, "unsupported"},
		{"missing role", func(m *metadata) { m.node.Capabilities["rel"] = []int{} }, "unsupported"},
		{"contradictory input", func(m *metadata) { m.node.InputID.Bus = 5 }, "indeterminate"},
		{"missing ancestry", func(m *metadata) { m.node.USBParent = nil }, "indeterminate"},
		{"unavailable metadata", func(m *metadata) { m.failure(errMetadata) }, "indeterminate"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := fixtureCollector(t)
			path := fixtureNode(t, c, "unit", 1, 1)
			m := c.metadata(path, filepath.Base(path))
			tc.change(&m)
			admit(&m)
			if m.node.Admission != tc.admission {
				t.Fatalf("got %s want %s", m.node.Admission, tc.admission)
			}
		})
	}
}

func TestMatchRelevance(t *testing.T) {
	t.Parallel()
	c := fixtureCollector(t)
	path := fixtureNode(t, c, "unit", 1, 1)
	m := c.metadata(path, filepath.Base(path))
	parent := *m.node.USBParent
	m.node.USBID.Vendor = 0x1234
	m.node.InputID.Vendor = 0x1234
	if relevant(m, nil) {
		t.Fatal("same-name counterfeit relevant")
	}
	if !relevant(m, map[string]bool{parent: true}) {
		t.Fatal("candidate sibling lost")
	}
	m.node.USBID = nil
	m.node.USBParent = nil
	if relevant(m, nil) {
		t.Fatal("non-USB unrelated input relevant")
	}
	m.node.InputID = nil
	if !relevant(m, nil) {
		t.Fatal("unknown relevance silently dismissed")
	}
}
