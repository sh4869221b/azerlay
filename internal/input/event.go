// Package input reduces selected evdev node events into immutable snapshots.
package input

import "time"

const (
	EV_SYN = 0x00
	EV_KEY = 0x01
	EV_REL = 0x02
	EV_ABS = 0x03
	EV_MSC = 0x04

	SYN_REPORT  = 0x00
	SYN_DROPPED = 0x03
	MSC_SCAN    = 0x04
)

// Event contains one raw evdev value. Timestamp is monotonic elapsed time,
// not Unix wall time. Node identity is supplied separately by the session.
type Event struct {
	Timestamp time.Duration
	Type      uint16
	Code      uint16
	Value     int32
}

type KeyAction int32

const (
	KeyRelease KeyAction = iota
	KeyPress
	KeyRepeat
)

// KeyTransition preserves each key change in reporting-frame order.
type KeyTransition struct {
	Timestamp time.Duration
	Code      uint16
	Action    KeyAction
}

// Generations are caller-supplied values fixed for the lifetime of a session.
type Generations struct {
	Device  uint64
	Profile uint64
}
