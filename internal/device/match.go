package device

import "slices"

// relevant separates candidate discovery from admission. A valid unrelated
// input ID is sufficient to dismiss a non-USB input node.
func relevant(m metadata, parents map[string]bool) bool {
	n := m.node
	if n.USBParent != nil && parents[*n.USBParent] {
		return true
	}
	if n.USBID != nil && n.USBID.Vendor == 0x16d0 && n.USBID.Product == 0x12f7 {
		return true
	}
	if n.InputID != nil && n.InputID.Vendor == 0x16d0 && n.InputID.Product == 0x12f7 {
		return true
	}
	return m.identityUnknown || n.USBID == nil && n.InputID == nil
}

func admit(m *metadata) {
	n := &m.node
	if len(m.findings) > 0 {
		return
	}
	if n.USBID == nil || n.USBParent == nil || n.Interface == nil {
		m.failure(errMetadata)
		return
	}
	if n.InputID != nil && *n.InputID != (InputID{3, 0x16d0, 0x12f7, 0x111}) {
		// A consistently unrelated device is unsupported; disagreement with a
		// qualifying USB signature is unavailable, contradictory evidence.
		if *n.USBID == (USBID{0x16d0, 0x12f7, 0x111}) {
			m.failure(errMetadata)
			return
		}
	}
	n.Admission = "unsupported"
	if *n.USBID != (USBID{0x16d0, 0x12f7, 0x111}) {
		return
	}
	i := *n.Interface
	var types, keys, rel, abs []int
	switch i.Number {
	case 1:
		if i != (Interface{1, 3, 0, 0}) {
			return
		}
		types, keys, rel, abs = []int{0, 1, 2, 3}, []int{30}, []int{6}, []int{0, 1}
		n.Roles = []string{"keyboard", "relative", "absolute"}
	case 2:
		if i != (Interface{2, 3, 1, 2}) {
			return
		}
		types, keys, rel = []int{0, 1, 2}, []int{272}, []int{0, 1}
		n.Roles = []string{"mouse_buttons", "relative"}
	case 3:
		if i != (Interface{3, 3, 0, 0}) {
			return
		}
		types, keys, abs = []int{0, 1, 3}, []int{288}, []int{0, 1}
		n.Roles = []string{"joystick_buttons", "absolute"}
	default:
		return
	}
	for family, required := range map[string][]int{"ev": types, "key": keys, "rel": rel, "abs": abs} {
		for _, code := range required {
			if !slices.Contains(n.Capabilities[family], code) {
				n.Roles = []string{}
				return
			}
		}
	}
	n.Admission = "admitted"
}
