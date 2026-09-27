package device

import "errors"

// ReconnectTarget pins USB identity independently of the transient hidraw number.
type ReconnectTarget struct {
	parent string
	usb    USBID
	serial string
}

func NewReconnectTarget(result *Result, selected Group) (ReconnectTarget, error) {
	node, err := reconnectNode(result, selected)
	if err != nil {
		return ReconnectTarget{}, err
	}
	target := ReconnectTarget{selected.USBParent, *node.USBID, nodeSerial(node)}
	if _, err := target.Select(result); err != nil {
		return ReconnectTarget{}, err
	}
	return target, nil
}
func (t ReconnectTarget) Valid() bool { return t.parent != "" }
func (t ReconnectTarget) Select(result *Result) (Group, error) {
	if !t.Valid() || result == nil {
		return Group{}, reconnectDiagnostic(ERR_DEVICE_METADATA)
	}
	var selected *Group
	for _, group := range result.Groups {
		matching := false
		for _, node := range result.Nodes {
			if node.USBParent != nil && *node.USBParent == group.USBParent && t.matches(node) {
				matching = true
			}
		}
		if !matching {
			continue
		}
		if selected != nil || len(group.HIDPaths) != 1 {
			return Group{}, reconnectDiagnostic(ERR_DEVICE_AMBIGUOUS)
		}
		if _, err := reconnectNode(result, group); err != nil {
			return Group{}, err
		}
		copied := group
		copied.HIDPaths = append([]string(nil), group.HIDPaths...)
		selected = &copied
	}
	if selected == nil {
		return Group{}, reconnectDiagnostic(ERR_DEVICE_NOT_FOUND)
	}
	return *selected, nil
}
func (t ReconnectTarget) Open(group Group) ([]OpenedNode, error) { return t.open(group, OpenGroup) }
func (t ReconnectTarget) open(group Group, open func(Group) ([]OpenedNode, error)) ([]OpenedNode, error) {
	if !t.Valid() || len(group.HIDPaths) != 1 || t.serial == "" && group.USBParent != t.parent {
		return nil, reconnectDiagnostic(ERR_DEVICE_METADATA)
	}
	opened, err := open(group)
	if err != nil {
		return nil, err
	}
	if len(opened) == 1 && t.matches(opened[0].Node) {
		return opened, nil
	}
	err = reconnectDiagnostic(ERR_DEVICE_METADATA)
	for _, node := range opened {
		if closeErr := node.File.Close(); closeErr != nil {
			d := boundaryDiagnostic(closeErr, "close", nil)
			err = errors.Join(err, &d)
		}
	}
	return nil, err
}
func (t ReconnectTarget) matches(node Node) bool {
	if node.USBID == nil || *node.USBID != t.usb || node.USBParent == nil {
		return false
	}
	if t.serial != "" {
		return nodeSerial(node) == t.serial
	}
	return *node.USBParent == t.parent
}
func nodeSerial(node Node) string {
	if node.Serial == nil {
		return ""
	}
	return *node.Serial
}
func reconnectDiagnostic(code string) error { d := diagnostic(code, "reconnect", nil); return &d }
func reconnectNode(result *Result, selected Group) (Node, error) {
	if result == nil || selected.USBParent == "" || len(selected.HIDPaths) != 1 {
		return Node{}, reconnectDiagnostic(ERR_DEVICE_METADATA)
	}
	memberships := 0
	for _, group := range result.Groups {
		if group.USBParent == selected.USBParent && len(group.HIDPaths) == 1 && group.HIDPaths[0] == selected.HIDPaths[0] {
			memberships++
		}
	}
	if memberships != 1 {
		return Node{}, reconnectDiagnostic(ERR_DEVICE_METADATA)
	}
	var found *Node
	for _, node := range result.Nodes {
		if node.Path != selected.HIDPaths[0] {
			continue
		}
		if found != nil {
			return Node{}, reconnectDiagnostic(ERR_DEVICE_AMBIGUOUS)
		}
		m := metadata{node: node}
		admit(&m)
		if node.USBParent == nil || *node.USBParent != selected.USBParent || node.Admission != "admitted" || m.node.Admission != "admitted" {
			return Node{}, reconnectDiagnostic(ERR_DEVICE_UNSUPPORTED)
		}
		copied := node
		found = &copied
	}
	if found == nil {
		return Node{}, reconnectDiagnostic(ERR_DEVICE_NOT_FOUND)
	}
	return *found, nil
}
