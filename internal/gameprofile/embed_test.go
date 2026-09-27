package gameprofile

import "testing"

func TestEmbeddedGeneric(t *testing.T) {
	t.Parallel()
	catalog, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	d, ok := catalog.Lookup("generic")
	if !ok || d.SchemaVersion != 1 || d.ID != "generic" || d.Name != "Generic" || len(d.Bindings) != 0 || len(d.Controls) != 0 {
		t.Fatalf("generic: %+v, found=%t", d, ok)
	}
}
