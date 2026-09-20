package device

import (
	"errors"
	"slices"
)

// ReconnectTarget pins a selected device and node order independently of event paths.
type ReconnectTarget struct {
	parent string
	usb    USBID
	serial string
	slots  []reconnectSlot
}

type reconnectSlot struct {
	iface Interface
	roles []string
}

func NewReconnectTarget(result *Result, selected Group) (ReconnectTarget, error) {
	nodes, err := reconnectNodes(result, selected)
	if err != nil {
		return ReconnectTarget{}, err
	}
	target := ReconnectTarget{parent: selected.USBParent, usb: *nodes[0].USBID, serial: nodeSerial(nodes[0])}
	for _, node := range nodes {
		slot := reconnectSlot{iface: *node.Interface, roles: slices.Clone(node.Roles)}
		for _, existing := range target.slots {
			if existing.matches(node) {
				return ReconnectTarget{}, reconnectDiagnostic(ERR_DEVICE_AMBIGUOUS)
			}
		}
		target.slots = append(target.slots, slot)
	}
	return target, nil
}

func (t ReconnectTarget) Valid() bool { return len(t.slots) > 0 }

func (t ReconnectTarget) Select(result *Result) (Group, error) {
	if !t.Valid() || result == nil {
		return Group{}, reconnectDiagnostic(ERR_DEVICE_METADATA)
	}
	var candidate Group
	var members []Node
	for _, group := range result.Groups {
		nodes, err := reconnectNodes(result, group)
		if err != nil || !t.matches(nodes[0]) {
			continue
		}
		if members != nil {
			return Group{}, reconnectDiagnostic(ERR_DEVICE_AMBIGUOUS)
		}
		candidate, members = group, nodes
	}
	if members == nil {
		return Group{}, reconnectDiagnostic(ERR_DEVICE_NOT_FOUND)
	}
	selected := Group{USBParent: candidate.USBParent, Complete: candidate.Complete}
	for _, slot := range t.slots {
		path := ""
		for _, node := range members {
			if !slot.matches(node) {
				continue
			}
			if path != "" {
				return Group{}, reconnectDiagnostic(ERR_DEVICE_AMBIGUOUS)
			}
			path = node.Path
		}
		if path == "" {
			return Group{}, reconnectDiagnostic(ERR_DEVICE_NOT_FOUND)
		}
		selected.EventPaths = append(selected.EventPaths, path)
	}
	return selected, nil
}

func (t ReconnectTarget) Open(group Group) ([]OpenedNode, error) {
	return t.open(group, OpenGroup)
}

func (t ReconnectTarget) open(group Group, open func(Group) ([]OpenedNode, error)) ([]OpenedNode, error) {
	if !t.Valid() || len(group.EventPaths) != len(t.slots) || (t.serial == "" && group.USBParent != t.parent) {
		return nil, reconnectDiagnostic(ERR_DEVICE_METADATA)
	}
	opened, err := open(group)
	if err != nil {
		return nil, err
	}
	valid := len(opened) == len(t.slots)
	for i, node := range opened {
		if i >= len(t.slots) || !t.matches(node.Node) || !t.slots[i].matches(node.Node) {
			valid = false
		}
	}
	if valid {
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

func (s reconnectSlot) matches(node Node) bool {
	return node.Interface != nil && *node.Interface == s.iface && slices.Equal(node.Roles, s.roles)
}

func nodeSerial(node Node) string {
	if node.Serial == nil {
		return ""
	}
	return *node.Serial
}

func reconnectDiagnostic(code string) error {
	d := diagnostic(code, "reconnect", nil)
	return &d
}

func reconnectNodes(result *Result, selected Group) ([]Node, error) {
	if result == nil || selected.USBParent == "" || len(selected.EventPaths) == 0 {
		return nil, reconnectDiagnostic(ERR_DEVICE_METADATA)
	}
	var group *Group
	for i := range result.Groups {
		if result.Groups[i].USBParent == selected.USBParent {
			if group != nil {
				return nil, reconnectDiagnostic(ERR_DEVICE_METADATA)
			}
			group = &result.Groups[i]
		}
	}
	if group == nil {
		return nil, reconnectDiagnostic(ERR_DEVICE_METADATA)
	}
	nodes := make([]Node, 0, len(selected.EventPaths))
	seen := make(map[string]bool)
	for _, path := range selected.EventPaths {
		if path == "" || seen[path] {
			return nil, reconnectDiagnostic(ERR_DEVICE_METADATA)
		}
		seen[path] = true
		memberships := 0
		for _, member := range group.EventPaths {
			if member == path {
				memberships++
			}
		}
		if memberships != 1 {
			return nil, reconnectDiagnostic(ERR_DEVICE_METADATA)
		}
		var found *Node
		for i := range result.Nodes {
			if result.Nodes[i].Path == path {
				if found != nil {
					return nil, reconnectDiagnostic(ERR_DEVICE_METADATA)
				}
				found = &result.Nodes[i]
			}
		}
		if found == nil || found.USBID == nil || found.USBParent == nil || *found.USBParent != selected.USBParent || found.Interface == nil {
			return nil, reconnectDiagnostic(ERR_DEVICE_METADATA)
		}
		m := metadata{node: *found}
		admit(&m)
		if found.Admission != "admitted" || m.node.Admission != "admitted" || len(m.findings) > 0 || !slices.Equal(found.Roles, m.node.Roles) {
			return nil, reconnectDiagnostic(ERR_DEVICE_UNSUPPORTED)
		}
		if len(nodes) > 0 && (*found.USBID != *nodes[0].USBID || nodeSerial(*found) != nodeSerial(nodes[0])) {
			return nil, reconnectDiagnostic(ERR_DEVICE_METADATA)
		}
		nodes = append(nodes, *found)
	}
	return nodes, nil
}
