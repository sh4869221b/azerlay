package input

import (
	"fmt"
	"testing"
)

func physicalBytes(id, state, counter byte) []byte {
	b := make([]byte, 64)
	b[2], b[3], b[6], b[7], b[8] = 57, counter, 2, id, state
	return b
}
func TestPhysicalReportMapping(t *testing.T) {
	d, err := newPhysicalDecoder()
	if err != nil {
		t.Fatal(err)
	}
	expected := map[byte]string{4: "grid.c1.r1", 3: "grid.c1.r2", 2: "grid.c1.r3", 1: "grid.c1.r4", 37: "grid.c1.r5", 8: "grid.c2.r1", 7: "grid.c2.r2", 6: "grid.c2.r3", 5: "grid.c2.r4", 38: "grid.c2.r5", 12: "grid.c3.r1", 11: "grid.c3.r2", 10: "grid.c3.r3", 9: "grid.c3.r4", 13: "grid.c3.r5", 17: "grid.c4.r1", 16: "grid.c4.r2", 15: "grid.c4.r3", 14: "grid.c4.r4", 18: "grid.c4.r5", 36: "grid.side-left", 19: "grid.side-right", 28: "cluster.top", 29: "cluster.left", 22: "cluster.center", 31: "cluster.right", 30: "cluster.bottom", 41: "stick.right-upper", 20: "stick.right-lower", 23: "stick.below"}
	for id, region := range expected {
		for _, state := range []byte{0, 1} {
			t.Run(fmt.Sprintf("%d/%d", id, state), func(t *testing.T) {
				b := physicalBytes(id, state, 7)
				b[63] = 255
				e, relevant, err := d.decode(b)
				if err != nil || !relevant || e.RegionID != region || e.Down != (state == 1) {
					t.Fatalf("%+v relevant=%v error=%v", e, relevant, err)
				}
			})
		}
	}
}
func TestPhysicalReportBoundary(t *testing.T) {
	d, err := newPhysicalDecoder()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		data []byte
	}{{"short", []byte{0, 0}}, {"truncated", physicalBytes(4, 1, 1)[:63]}, {"overlong", append(physicalBytes(4, 1, 1), 0)}, {"unknown source", physicalBytes(255, 1, 1)}, {"invalid state", physicalBytes(4, 2, 1)}} {
		t.Run(tc.name, func(t *testing.T) {
			_, known, err := d.decode(tc.data)
			if err == nil || known {
				t.Fatal("malformed accepted")
			}
		})
	}
	b := physicalBytes(4, 1, 1)
	b[6] = 3
	if _, known, err := d.decode(b); known || err == nil {
		t.Fatal("declared length ignored")
	}
	b[2] = 18
	if _, known, err := d.decode(b); known || err != nil {
		t.Fatal("other type interpreted")
	}
}
