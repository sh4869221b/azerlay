package device

// RawID is the identity held by the kernel HID driver.
type RawID struct {
	Bus             uint32
	Vendor, Product uint16
}

type ReportDescriptor struct {
	UsagePage   uint16 `json:"usage_page"`
	Usage       uint16 `json:"usage"`
	ReportBytes int    `json:"report_bytes"`
	Numbered    bool   `json:"numbered"`
}

type USBID struct {
	Vendor, Product, Release uint16
}

type Interface struct {
	Number, Class, Subclass, Protocol uint8
}

type Node struct {
	Path             string
	USBParent        *string
	Interface        *Interface
	Roles            []string
	Admission        string
	Access           string
	Name             *string
	ReportDescriptor *ReportDescriptor
	USBID            *USBID
	SysfsPath        *string
	PhysicalPath     *string
	Serial           *string
	deviceNumber     uint64
}

type Group struct {
	USBParent string
	Complete  bool
	HIDPaths  []string
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
