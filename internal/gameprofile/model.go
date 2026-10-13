package gameprofile

import (
	"maps"

	"github.com/sh4869221b/azerlay/internal/profile"
)

type ControlKey struct {
	InputID int
	Trigger profile.TriggerKind
}

type Definition struct {
	SchemaVersion int
	ID            string
	Name          string
	Locale        string
	Controls      map[ControlKey]string
}

type Catalog struct {
	definitions map[string]Definition
}

func (c Catalog) Lookup(id string) (Definition, bool) {
	d, ok := c.definitions[id]
	if !ok {
		return Definition{}, false
	}
	d.Controls = maps.Clone(d.Controls)
	return d, true
}
