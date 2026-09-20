package input

// AxisInfo describes the kernel's absolute-axis range and filtering metadata.
type AxisInfo struct {
	Minimum int32
	Maximum int32
	Flat    int32
	Fuzz    int32
}

// NormalizeAxis applies the kernel range and flat zone without changing raw.
// Fuzz is metadata only: the input core has already filtered reported values.
func NormalizeAxis(raw int32, info AxisInfo) (float64, bool) {
	minimum, maximum := float64(info.Minimum), float64(info.Maximum)
	center := (minimum + maximum) / 2
	left, right, flat := center-minimum, maximum-center, float64(info.Flat)
	if minimum >= maximum || flat < 0 || info.Fuzz < 0 || flat >= left || flat >= right {
		return 0, false
	}
	delta := float64(raw) - center
	if delta > flat {
		return min(1, (delta-flat)/(right-flat)), true
	}
	if delta < -flat {
		return max(-1, (delta+flat)/(left-flat)), true
	}
	return 0, true
}

// NormalizedAbsolute distinguishes an observed raw value from a usable
// normalization. Missing observations or metadata leave valid false.
func (s *Snapshot) NormalizedAbsolute(node int, code uint16) (raw int32, normalized float64, known, valid bool) {
	raw, known = s.Absolute(node, code)
	info, present := s.AxisInfo(node, code)
	if known && present {
		normalized, valid = NormalizeAxis(raw, info)
	}
	return
}
