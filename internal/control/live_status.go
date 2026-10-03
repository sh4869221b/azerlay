package control

// LiveStatus is one coherent runtime publication. Providers perform no I/O or
// GTK operations; metrics reads may rotate their fixed measurement windows.
type LiveStatus struct {
	Device                *string
	EventRate, RenderRate *float64
	DroppedCount          *uint64
	Diagnostics           []Diagnostic
}

func (c *Controller) SetLiveStatusProvider(provider func() *LiveStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.liveStatus = provider
}

func (c *Controller) RuntimeGeneration() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.generation
}
func (c *Controller) AdvanceRuntimeGeneration() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.generation++
	return c.generation
}
