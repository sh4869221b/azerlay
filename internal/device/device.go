package device

// InputID is the kernel input_id, not a persistent device identity.
type InputID struct {
	Bus, Vendor, Product, Version uint16
}

type USBID struct {
	Vendor, Product, Release uint16
}

type Interface struct {
	Number, Class, Subclass, Protocol uint8
}

type Node struct {
	Path         string
	USBParent    *string
	Interface    *Interface
	Roles        []string
	Admission    string
	Access       string
	Name         *string
	InputID      *InputID
	USBID        *USBID
	SysfsPath    *string
	PhysicalPath *string
	Serial       *string
	Capabilities map[string][]int
	deviceNumber uint64
}

type Group struct {
	USBParent  string
	Complete   bool
	EventPaths []string
}

type Result struct {
	Groups      []Group
	Nodes       []Node
	Diagnostics []Diagnostic
}

// OK requires a readable admitted node and no error diagnostic.
func (r *Result) OK() bool {
	readable := false
	for _, n := range r.Nodes {
		readable = readable || n.Admission == "admitted" && n.Access == "readable"
	}
	for _, d := range r.Diagnostics {
		if d.Severity == "error" {
			return false
		}
	}
	return readable
}

type probeResult struct {
	name          *string
	access        string
	contradiction bool
	findings      []Diagnostic
}
