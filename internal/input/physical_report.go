// Package input publishes last-observed physical button state from qualified HID reports.
package input

import "github.com/sh4869221b/azerlay/internal/layout"

type PhysicalEvent struct {
	SourceID uint8
	RegionID string
	Down     bool
	counter  uint8
}

type physicalDecoder struct{ regions map[uint8]string }

func newPhysicalDecoder() (physicalDecoder, error) {
	definition, err := layout.LoadEmbedded("cyborg-ii", "left")
	if err != nil {
		return physicalDecoder{}, err
	}
	regions := make(map[uint8]string, 30)
	for _, control := range definition.Controls {
		if control.ID != "stick.main" {
			regions[uint8(control.SourceInputID)] = control.ID
		}
	}
	return physicalDecoder{regions}, nil
}

// decode ignores unrelated vendor types and padding. Invalid physical reports
// invalidate observation knowledge; they never imply a release.
func (d physicalDecoder) decode(report []byte) (PhysicalEvent, bool, error) {
	if len(report) < 3 {
		return PhysicalEvent{}, false, &InputError{Code: ERR_INPUT_EVENT}
	}
	if report[2] != 57 {
		return PhysicalEvent{}, false, nil
	}
	if len(report) != 64 || report[6] != 2 || report[8] > 1 {
		return PhysicalEvent{}, false, &InputError{Code: ERR_INPUT_EVENT}
	}
	region, ok := d.regions[report[7]]
	if !ok {
		return PhysicalEvent{}, false, &InputError{Code: ERR_INPUT_EVENT}
	}
	return PhysicalEvent{SourceID: report[7], RegionID: region, Down: report[8] == 1, counter: report[3]}, true, nil
}
