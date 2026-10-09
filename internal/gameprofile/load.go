package gameprofile

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/sh4869221b/azerlay/internal/profile"
)

type document struct {
	SchemaVersion int               `toml:"schema_version"`
	ID            string            `toml:"id"`
	Name          string            `toml:"name"`
	Locale        string            `toml:"locale"`
	Controls      map[string]string `toml:"controls"`
}

func Load(userDir string) (Catalog, error) {
	if userDir == "" {
		var err error
		userDir, err = ResolveGamesDir()
		if err != nil {
			return Catalog{}, err
		}
	}
	return loadFromFS(embedded, userDir)
}

func loadFromFS(builtIn fs.FS, userDir string) (Catalog, error) {
	catalog := Catalog{definitions: make(map[string]Definition)}
	entries, err := fs.ReadDir(builtIn, "assets")
	if err != nil {
		return Catalog{}, invalid("embedded", "embedded assets unavailable")
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".toml" {
			continue
		}
		data, err := fs.ReadFile(builtIn, "assets/"+entry.Name())
		if err != nil {
			return Catalog{}, invalid("embedded", "embedded asset read failed")
		}
		d, err := parse(data)
		if err != nil {
			return Catalog{}, err
		}
		if _, exists := catalog.definitions[d.ID]; exists {
			return Catalog{}, invalid("duplicate", "duplicate embedded ID")
		}
		catalog.definitions[d.ID] = d
	}
	entries, err = os.ReadDir(userDir)
	if errors.Is(err, os.ErrNotExist) {
		return catalog, nil
	}
	if err != nil {
		return Catalog{}, invalid("read", "user directory read failed")
	}
	seen := make(map[string]bool)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".toml" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(userDir, entry.Name()))
		if err != nil {
			return Catalog{}, invalid("read", "user profile read failed")
		}
		d, err := parse(data)
		if err != nil {
			return Catalog{}, err
		}
		if seen[d.ID] {
			return Catalog{}, invalid("duplicate", "duplicate user ID")
		}
		seen[d.ID] = true
		catalog.definitions[d.ID] = d
	}
	return catalog, nil
}

func parse(data []byte) (Definition, error) {
	var raw document
	if err := toml.NewDecoder(bytes.NewReader(data)).DisallowUnknownFields().Decode(&raw); err != nil {
		return Definition{}, invalid("decode", "invalid TOML or field type")
	}
	if raw.SchemaVersion != 1 {
		return Definition{}, invalid("schema", "schema_version must be 1")
	}
	if strings.TrimSpace(raw.ID) == "" || strings.TrimSpace(raw.Name) == "" {
		return Definition{}, invalid("validation", "id and name must be non-empty")
	}
	d := Definition{SchemaVersion: raw.SchemaVersion, ID: raw.ID, Name: raw.Name, Locale: raw.Locale,
		Controls: make(map[ControlKey]string, len(raw.Controls))}
	for key, value := range raw.Controls {
		parts := strings.Split(key, ":")
		if len(parts) != 3 || parts[0] != "input" {
			return Definition{}, invalid("validation", "invalid control key")
		}
		id, err := strconv.Atoi(parts[1])
		if err != nil || id <= 0 || strconv.Itoa(id) != parts[1] {
			return Definition{}, invalid("validation", "invalid control key")
		}
		trigger := profile.TriggerKind(parts[2])
		if !slices.Contains([]profile.TriggerKind{profile.TriggerSingle, profile.TriggerLong, profile.TriggerDouble}, trigger) {
			return Definition{}, invalid("validation", "invalid control key")
		}
		d.Controls[ControlKey{InputID: id, Trigger: trigger}] = value
	}
	return d, nil
}
