package layershell

import "fmt"

type PlacementInput struct {
	Anchor        string
	MarginX       int
	MarginY       int
	MonitorWidth  int
	MonitorHeight int
	WindowWidth   int
	WindowHeight  int
	HasGeometry   bool
}

type Placement struct {
	Left, Right, Top, Bottom bool
	LeftMargin, RightMargin  int
	TopMargin, BottomMargin  int
}

func Place(input PlacementInput) (Placement, error) {
	var p Placement
	switch input.Anchor {
	case "top-left":
		p.Left, p.Top = true, true
	case "top":
		p.Top = true
	case "top-right":
		p.Right, p.Top = true, true
	case "left":
		p.Left = true
	case "center":
	case "right":
		p.Right = true
	case "bottom-left":
		p.Left, p.Bottom = true, true
	case "bottom":
		p.Bottom = true
	case "bottom-right":
		p.Right, p.Bottom = true, true
	default:
		return Placement{}, fmt.Errorf("invalid overlay anchor %q", input.Anchor)
	}
	if input.HasGeometry {
		if input.MonitorWidth <= 0 || input.MonitorHeight <= 0 || input.WindowWidth <= 0 || input.WindowHeight <= 0 {
			return Placement{}, fmt.Errorf("invalid placement geometry")
		}
		if !p.Left && !p.Right {
			p.Left = true
			input.MarginX += (input.MonitorWidth - input.WindowWidth) / 2
		}
		if !p.Top && !p.Bottom {
			p.Top = true
			input.MarginY += (input.MonitorHeight - input.WindowHeight) / 2
		}
	}
	if p.Right {
		p.RightMargin = input.MarginX
	} else {
		p.LeftMargin = input.MarginX
	}
	if p.Bottom {
		p.BottomMargin = input.MarginY
	} else {
		p.TopMargin = input.MarginY
	}
	return p, nil
}
