package live

import "time"

// Coalescer schedules only dirty render state; the GTK bridge owns its one-shot
// source. Placement and visibility bypass this render deadline.
type Coalescer struct{ last time.Time }

func (c *Coalescer) Delay(now time.Time, hz int) time.Duration {
	if c.last.IsZero() {
		return 0
	}
	delay := c.last.Add(time.Second / time.Duration(hz)).Sub(now)
	if delay <= 0 {
		return 0
	}
	return ((delay + time.Millisecond - 1) / time.Millisecond) * time.Millisecond
}

func (c *Coalescer) Applied(now time.Time) { c.last = now }
