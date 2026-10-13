package device

import (
	"errors"
	"os"
	"syscall"
)

// OpenedNode pairs fresh admitted metadata with the verified descriptor.
// The caller owns File and must close it.
type OpenedNode struct {
	Node Node
	File *os.File
}

// OpenGroup opens the single selected interface04 path.
// On failure no descriptor is transferred to the caller.
func OpenGroup(group Group) ([]OpenedNode, error) {
	ops := probeOps{open: syscall.Open, fstat: syscall.Fstat, id: ioctlID, descriptor: ioctlDescriptor, close: syscall.Close}
	return systemCollector().openGroup(group, ops)
}

func (c collector) openGroup(group Group, ops probeOps) (opened []OpenedNode, err error) {
	if len(group.HIDPaths) != 1 || group.USBParent == "" {
		d := diagnostic(ERR_DEVICE_METADATA, "selection", nil)
		return nil, &d
	}
	path := group.HIDPaths[0]
	if path == "" {
		d := diagnostic(ERR_DEVICE_METADATA, "selection", &path)
		return nil, &d
	}
	n, err := c.selectedNode(path, group.USBParent)
	if err != nil {
		return nil, err
	}
	fd, err := ops.open(path, syscall.O_RDONLY|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		d := boundaryDiagnostic(err, "open", &path)
		return nil, &d
	}
	defer func() {
		if err != nil {
			if closeErr := ops.close(fd); closeErr != nil {
				d := boundaryDiagnostic(closeErr, "close", &path)
				err = errors.Join(err, &d)
			}
		}
	}()
	if _, err := ops.validate(fd, n); err != nil {
		d := boundaryDiagnostic(err, "open", &path)
		return nil, &d
	}
	// Opening can race with unplug/replacement. Recollect the current mapping
	// and admission, then compare that identity to the descriptor we retain.
	n, err = c.selectedNode(path, group.USBParent)
	if err != nil {
		return nil, err
	}
	if _, err := ops.validate(fd, n); err != nil {
		d := boundaryDiagnostic(err, "open", &path)
		return nil, &d
	}
	n.Access = "readable"
	return []OpenedNode{{Node: n, File: os.NewFile(uintptr(fd), path)}}, nil
}

func (c collector) selectedNode(path, parent string) (Node, error) {
	event, err := c.resolve(path)
	if err != nil {
		d := boundaryDiagnostic(err, "resolution", &path)
		return Node{}, &d
	}
	m := c.metadata(path, event)
	admit(&m)
	if len(m.findings) > 0 {
		return Node{}, &m.findings[0]
	}
	if m.node.Admission != "admitted" {
		d := diagnostic(ERR_DEVICE_UNSUPPORTED, "admission", &path)
		return Node{}, &d
	}
	if m.node.USBParent == nil || *m.node.USBParent != parent {
		d := diagnostic(ERR_DEVICE_METADATA, "selection", &path)
		return Node{}, &d
	}
	return m.node, nil
}
