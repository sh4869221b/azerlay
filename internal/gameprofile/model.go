package gameprofile

import "github.com/sh4869221b/azerlay/internal/profile"

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
	d.Controls = cloneMap(d.Controls)
	return d, true
}

func cloneMap[K comparable](src map[K]string) map[K]string {
	if src == nil {
		return nil
	}
	dst := make(map[K]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
