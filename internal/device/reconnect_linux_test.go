package device

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRawReconnectOpenRejectsReplacementSerial(t *testing.T) {
	t.Parallel()
	c := fixtureCollector(t)
	fixtureNode(t, c, "unit", 1, 4)
	discovered, _ := c.collect("")
	group := discovered.Groups[0]
	target, err := NewReconnectTarget(discovered, group)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(group.HIDPaths[0])
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	replacement := discovered.Nodes[0]
	serial := "replacement"
	replacement.Serial = &serial
	opened, err := target.open(group, func(Group) ([]OpenedNode, error) {
		return []OpenedNode{{File: file, Node: replacement}}, nil
	})
	var diagnostic *Diagnostic
	if opened != nil || !errors.As(err, &diagnostic) || diagnostic.Code != ERR_DEVICE_METADATA {
		t.Fatalf("replacement admitted: %v, %v", opened, err)
	}
	if _, err := file.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("replacement descriptor leaked: %v", err)
	}
}

func TestRawReconnectIdentity(t *testing.T) {
	for _, serial := range []bool{true, false} {
		t.Run(map[bool]string{true: "serial", false: "same-parent"}[serial], func(t *testing.T) {
			c := fixtureCollector(t)
			fixtureNode(t, c, "unit", 1, 4)
			if !serial {
				if err := os.Remove(filepath.Join(c.sysRoot, "devices/unit/serial")); err != nil {
					t.Fatal(err)
				}
			}
			r, _ := c.collect("")
			target, err := NewReconnectTarget(r, r.Groups[0])
			if err != nil {
				t.Fatal(err)
			}
			r.Nodes[0].Path = filepath.Join(c.devRoot, "hidraw99")
			r.Groups[0].HIDPaths = []string{r.Nodes[0].Path}
			selected, err := target.Select(r)
			if err != nil || selected.HIDPaths[0] != r.Nodes[0].Path {
				t.Fatalf("%+v %v", selected, err)
			}
			selected.HIDPaths[0] = "mutated"
			if r.Groups[0].HIDPaths[0] == "mutated" {
				t.Fatal("mutable result")
			}
			parent := "other-parent"
			r.Nodes[0].USBParent = &parent
			r.Groups[0].USBParent = parent
			_, err = target.Select(r)
			if serial && err != nil || !serial && err == nil {
				t.Fatalf("identity selection: %v", err)
			}
		})
	}
}
func TestRawReconnectAmbiguity(t *testing.T) {
	c := fixtureCollector(t)
	fixtureNode(t, c, "unit", 1, 4)
	r, _ := c.collect("")
	target, err := NewReconnectTarget(r, r.Groups[0])
	if err != nil {
		t.Fatal(err)
	}
	fixtureNode(t, c, "second", 2, 4)
	r, _ = c.collect("")
	if _, err := target.Select(r); err == nil || err.Error() != ERR_DEVICE_AMBIGUOUS {
		t.Fatalf("collision=%v", err)
	}
	c = fixtureCollector(t)
	fixtureNode(t, c, "unit", 1, 4)
	fixtureNode(t, c, "unit", 2, 4)
	r, _ = c.collect("")
	if r.Groups[0].Complete {
		t.Fatal("duplicate interface complete")
	}
	if _, err := NewReconnectTarget(r, r.Groups[0]); err == nil {
		t.Fatal("duplicate admitted")
	}
}
