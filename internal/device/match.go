package device

const physicalDescriptor = "\x06\x01\xff\x0a\x01\x01\xa1\x01\x75\x08\x15\x00\x26\xff\x00\x95\x40\x09\x01\x81\x02\x95\x40\x09\x02\x91\x02\xc0"

func relevant(m metadata, parents map[string]bool) bool {
	n := m.node
	if n.USBParent != nil && parents[*n.USBParent] {
		return true
	}
	return m.identityUnknown || n.USBID == nil || n.USBID.Vendor == 0x16d0 && n.USBID.Product == 0x12f7
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
	n.Admission = "unsupported"
	if *n.USBID != (USBID{0x16d0, 0x12f7, 0x111}) || *n.Interface != (Interface{4, 3, 0, 0}) || n.ReportDescriptor == nil || *n.ReportDescriptor != (ReportDescriptor{0xff01, 0x101, 64, false}) {
		return
	}
	n.Roles = []string{"physical-buttons"}
	n.Admission = "admitted"
}
