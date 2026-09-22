package layout

type Definition struct {
	SchemaVersion int           `json:"schema_version"`
	Model         string        `json:"model"`
	Hand          string        `json:"hand"`
	Applicability Applicability `json:"applicability"`
	ViewBox       Rect          `json:"view_box"`
	Controls      []Control     `json:"controls"`
	Decorations   []Shape       `json:"decorations,omitempty"`
}

type Applicability struct {
	SoftwareRelease   string  `json:"software_release"`
	DisplayedFirmware string  `json:"displayed_firmware"`
	HardwareRevision  *string `json:"hardware_revision"`
	Mode              string  `json:"mode"`
}

type Rect struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Control struct {
	ID            string `json:"id"`
	Group         string `json:"group"`
	SourceInputID int    `json:"source_input_id"`
	PinOne        int    `json:"pin_one"`
	PinTwo        int    `json:"pin_two"`
	ZIndex        int    `json:"z_index"`
	Shape         Shape  `json:"shape"`
	LabelAnchor   Point  `json:"label_anchor"`
}

type Shape struct {
	Type      string        `json:"type"`
	X         float64       `json:"x,omitempty"`
	Y         float64       `json:"y,omitempty"`
	Width     float64       `json:"width,omitempty"`
	Height    float64       `json:"height,omitempty"`
	Radius    float64       `json:"radius,omitempty"`
	CX        float64       `json:"cx,omitempty"`
	CY        float64       `json:"cy,omitempty"`
	RX        float64       `json:"rx,omitempty"`
	RY        float64       `json:"ry,omitempty"`
	Points    []Point       `json:"points,omitempty"`
	From      Point         `json:"from,omitempty"`
	To        Point         `json:"to,omitempty"`
	Commands  []PathCommand `json:"commands,omitempty"`
	Transform Transform     `json:"transform,omitempty"`
	Children  []Shape       `json:"children,omitempty"`
}

type PathCommand struct {
	Op     string  `json:"op"`
	Points []Point `json:"points"`
}

type Transform struct {
	TranslateX float64 `json:"translate_x"`
	TranslateY float64 `json:"translate_y"`
	ScaleX     float64 `json:"scale_x"`
	ScaleY     float64 `json:"scale_y"`
}
