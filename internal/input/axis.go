package input

// AxisInfo describes the kernel's absolute-axis range and filtering metadata.
type AxisInfo struct {
	Minimum int32
	Maximum int32
	Flat    int32
	Fuzz    int32
}
