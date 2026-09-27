package gameprofile

type Store struct {
	catalog Catalog
}

func (s *Store) Reload(userDir string) error {
	candidate, err := Load(userDir)
	if err != nil {
		return err
	}
	s.catalog = candidate
	return nil
}

func (s *Store) Snapshot() Catalog {
	return s.catalog
}
