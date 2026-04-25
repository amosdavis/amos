// Package mib loads and manages SNMP MIB files, building an OID tree for the UI.
package mib

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/sleepinggenius2/gosmi"
	"github.com/sleepinggenius2/gosmi/types"
)

// Node represents a single entry in the OID tree.
type Node struct {
	OID         string
	NumericOID  string
	Name        string
	Syntax      string
	Access      string
	Description string
	Children    []*Node
	Parent      *Node
}

// LoadError records a MIB file that failed to load.
type LoadError struct {
	File string
	Err  error
}

// Loader manages MIB paths and provides a built OID tree.
type Loader struct {
	mu       sync.RWMutex
	paths    []string
	root     *Node
	oidMap   map[string]*Node // numeric OID → Node
	nameMap  map[string]*Node // name → Node
	errors   []LoadError
	loaded   bool
}

// NewLoader creates a Loader pre-seeded with a directory of bundled MIBs.
func NewLoader(bundleDir string) *Loader {
	return &Loader{
		paths:   []string{bundleDir},
		oidMap:  make(map[string]*Node),
		nameMap: make(map[string]*Node),
	}
}

// AddPath appends an additional directory to search for MIB files.
func (l *Loader) AddPath(dir string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.paths = append(l.paths, dir)
}

// Load (re)loads all MIBs from all registered paths.
// It is safe to call multiple times; the tree is rebuilt on each call.
func (l *Loader) Load() []LoadError {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.root = &Node{Name: "iso", OID: "iso", NumericOID: "1"}
	l.oidMap = make(map[string]*Node)
	l.nameMap = make(map[string]*Node)
	l.errors = nil

	// gosmi operates on a global module registry.
	gosmi.Init()
	for _, dir := range l.paths {
		gosmi.AppendPath(dir)
	}

	// Enumerate all .txt / no-extension MIB files in the paths.
	for _, dir := range l.paths {
		entries, err := os.ReadDir(dir)
		if err != nil {
			l.errors = append(l.errors, LoadError{File: dir, Err: err})
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			// gosmi uses the filename stem as the module name.
			module := strings.TrimSuffix(name, filepath.Ext(name))
			l.loadModule(module)
		}
	}

	l.loaded = true
	return l.errors
}

// loadModule loads a single MIB module by name, recovering from panics (T-02).
func (l *Loader) loadModule(module string) {
	defer func() {
		if r := recover(); r != nil {
			l.errors = append(l.errors, LoadError{
				File: module,
				Err:  fmt.Errorf("panic: %v", r),
			})
		}
	}()

	m, err := gosmi.GetModule(module)
	if err != nil {
		// Try loading it first.
		if _, lerr := gosmi.LoadModule(module); lerr != nil {
			l.errors = append(l.errors, LoadError{File: module, Err: lerr})
			return
		}
		m, err = gosmi.GetModule(module)
		if err != nil {
			l.errors = append(l.errors, LoadError{File: module, Err: err})
			return
		}
	}

	nodes := m.GetNodes()
	for _, n := range nodes {
		l.addNode(n)
	}
}

func (l *Loader) addNode(sn gosmi.SmiNode) {
	numericOID := sn.RenderNumeric()
	name := sn.Name

	node := &Node{
		OID:        name,
		NumericOID: numericOID,
		Name:       name,
		Description: sn.Description,
	}

	if sn.Type != nil {
		node.Syntax = sn.Type.Name
	}
	if sn.Access != types.AccessUnknown {
		node.Access = sn.Access.String()
	}

	l.oidMap[numericOID] = node
	l.nameMap[strings.ToLower(name)] = node
}

// BuildTree assembles the flat OID map into a tree rooted at iso (1).
// Must be called after Load().
func (l *Loader) BuildTree() *Node {
	l.mu.RLock()
	defer l.mu.RUnlock()

	root := &Node{Name: "iso", NumericOID: "1", OID: "iso"}
	nodesByOID := map[string]*Node{"1": root}

	// Collect and sort OIDs so parents are always inserted before children.
	oids := make([]string, 0, len(l.oidMap))
	for oid := range l.oidMap {
		oids = append(oids, oid)
	}
	sort.Slice(oids, func(i, j int) bool {
		return compareOIDs(oids[i], oids[j]) < 0
	})

	for _, oid := range oids {
		node := l.oidMap[oid]
		parent := findOrCreateParent(oid, nodesByOID)
		if parent != nil {
			node.Parent = parent
			parent.Children = append(parent.Children, node)
		}
		nodesByOID[oid] = node
	}

	return root
}

// ResolveOID returns the Node for a numeric OID, or nil.
// Falls back to a synthetic node with just the numeric OID (T-03).
// Scalar instance identifiers (trailing .0) are stripped and retried.
func (l *Loader) ResolveOID(numericOID string) *Node {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if n, ok := l.oidMap[numericOID]; ok {
		return n
	}
	// Strip trailing .0 (scalar instance identifier) and retry.
	if strings.HasSuffix(numericOID, ".0") {
		base := numericOID[:len(numericOID)-2]
		if n, ok := l.oidMap[base]; ok {
			return n
		}
	}
	// Numeric-only fallback (T-03).
	return &Node{OID: numericOID, NumericOID: numericOID, Name: numericOID}
}

// ResolveByName returns the Node for a MIB object name (case-insensitive), or nil.
func (l *Loader) ResolveByName(name string) *Node {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.nameMap[strings.ToLower(name)]
}

// Errors returns any load errors from the last Load() call.
func (l *Loader) Errors() []LoadError {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return append([]LoadError(nil), l.errors...)
}

// findOrCreateParent finds the direct parent node for a given numeric OID,
// creating synthetic intermediate nodes as needed.
func findOrCreateParent(oid string, nodes map[string]*Node) *Node {
	lastDot := strings.LastIndex(oid, ".")
	if lastDot < 0 {
		return nil
	}
	parentOID := oid[:lastDot]
	if p, ok := nodes[parentOID]; ok {
		return p
	}
	// Synthetic parent.
	p := &Node{NumericOID: parentOID, Name: parentOID, OID: parentOID}
	nodes[parentOID] = p
	grandparent := findOrCreateParent(parentOID, nodes)
	if grandparent != nil {
		p.Parent = grandparent
		grandparent.Children = append(grandparent.Children, p)
	}
	return p
}

// compareOIDs compares two dotted-numeric OID strings lexicographically by segment.
func compareOIDs(a, b string) int {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")
	for i := 0; i < len(partsA) && i < len(partsB); i++ {
		if partsA[i] < partsB[i] {
			return -1
		}
		if partsA[i] > partsB[i] {
			return 1
		}
	}
	return len(partsA) - len(partsB)
}
