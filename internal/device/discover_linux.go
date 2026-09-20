package device

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

func systemCollector() collector {
	return collector{devRoot: "/dev/input", sysRoot: "/sys", probe: probeNode, stat: syscall.Stat, uid: os.Geteuid()}
}

// Discover collects relevant metadata and probes only admitted event nodes.
func Discover() (*Result, *Diagnostic) { return systemCollector().collect("") }

// Inspect examines only path when supplied; empty path examines all candidates.
// The supplied path is a transient locator, including when it is a symlink.
func Inspect(path string) (*Result, *Diagnostic) { return systemCollector().collect(path) }

func (c collector) collect(path string) (*Result, *Diagnostic) {
	var entries []metadata
	if path != "" {
		event, err := c.resolve(path)
		if err != nil {
			d := boundaryDiagnostic(err, "resolution", &path)
			if errors.Is(err, fs.ErrNotExist) {
				d = diagnostic(ERR_DEVICE_NOT_FOUND, "resolution", &path)
			}
			return nil, &d
		}
		entries = []metadata{c.metadata(path, event)}
	} else {
		paths, err := c.events()
		if err != nil {
			d := boundaryDiagnostic(err, "enumeration", nil)
			return nil, &d
		}
		for _, p := range paths {
			entries = append(entries, c.metadata(p, filepath.Base(p)))
		}
	}
	r := &Result{Groups: []Group{}, Nodes: []Node{}, Diagnostics: []Diagnostic{}}
	parents := map[string]bool{}
	for _, m := range entries {
		if m.node.USBParent != nil && relevant(m, nil) {
			parents[*m.node.USBParent] = true
		}
	}
	for _, m := range entries {
		if path == "" && !relevant(m, parents) {
			continue
		}
		admit(&m)
		n := m.node
		r.Diagnostics = append(r.Diagnostics, m.findings...)
		if n.Admission == "unsupported" {
			d := diagnostic(ERR_DEVICE_UNSUPPORTED, "admission", &n.Path)
			if path == "" {
				d.Severity = "warning"
			}
			r.Diagnostics = append(r.Diagnostics, d)
		}
		if n.Admission == "admitted" {
			p := c.probe(n)
			n.Access = p.access
			if p.name != nil {
				n.Name = p.name
			}
			if p.contradiction {
				n.Admission = "indeterminate"
				n.Roles = []string{}
			}
			r.Diagnostics = append(r.Diagnostics, p.findings...)
		}
		r.Nodes = append(r.Nodes, n)
	}
	groups, findings := groupNodes(r.Nodes)
	r.Groups = groups
	r.Diagnostics = append(r.Diagnostics, findings...)
	if len(groups) == 0 {
		r.Diagnostics = append(r.Diagnostics, diagnostic(ERR_DEVICE_NOT_FOUND, "discovery", nil))
	}
	if c.uid == 0 {
		r.Diagnostics = append(r.Diagnostics, diagnostic(WARN_DEVICE_ROOT, "access", nil))
	}
	sortResult(r)
	return r, nil
}
