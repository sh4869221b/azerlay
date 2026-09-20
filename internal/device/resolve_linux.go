package device

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type collector struct {
	devRoot string
	sysRoot string
	probe   func(Node) probeResult
	stat    func(string, *syscall.Stat_t) error
	uid     int
}

func eventNumber(name string) (uint64, bool) {
	if !strings.HasPrefix(name, "event") {
		return 0, false
	}
	n, err := strconv.ParseUint(strings.TrimPrefix(name, "event"), 10, 32)
	return n, err == nil
}

func (c collector) events() ([]string, error) {
	entries, err := os.ReadDir(c.devRoot)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	paths := []string{}
	for _, entry := range entries {
		if _, valid := eventNumber(entry.Name()); valid {
			paths = append(paths, filepath.Join(c.devRoot, entry.Name()))
		}
	}
	return paths, nil
}

func (c collector) resolve(path string) (string, error) {
	var st syscall.Stat_t
	if err := c.stat(path, &st); err != nil {
		return "", err
	}
	if st.Mode&syscall.S_IFMT != syscall.S_IFCHR {
		return "", errMetadata
	}
	major := (st.Rdev>>8)&0xfff | (st.Rdev>>32)&0xfffff000
	minor := st.Rdev&0xff | (st.Rdev>>12)&0xffffff00
	resolved, err := filepath.EvalSymlinks(filepath.Join(c.sysRoot, "dev/char", fmt.Sprintf("%d:%d", major, minor)))
	if err != nil {
		return "", err
	}
	event := filepath.Base(resolved)
	if _, valid := eventNumber(event); !valid {
		return "", errMetadata
	}
	valid, err := subsystem(resolved, "input")
	if err != nil {
		return "", err
	}
	if !valid {
		return "", errMetadata
	}
	class, err := filepath.EvalSymlinks(filepath.Join(c.sysRoot, "class/input", event))
	if err != nil {
		return "", err
	}
	if class != resolved {
		return "", errMetadata
	}
	return event, nil
}
