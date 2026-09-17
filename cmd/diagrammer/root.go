package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xmhha/diagrammer/internal/command"
)

// Confining the server to a directory.
//
// A path typed at a terminal was chosen by the person who owns the terminal,
// and they can already write anywhere their shell can; restricting the command
// line would protect nobody from anyone. A path arriving over MCP was chosen by
// a plugin, or by a model the plugin is driving, and that model has just been
// handed the doc comments of a repository it did not write. Text from a
// stranger's source tree reaching an argument that names a file to overwrite is
// a short enough path to be worth closing.
//
// So the server takes a root and refuses anything outside it. The default is
// the directory it was started in, because a safe default that can be widened
// is worth more than a wide default that can be narrowed: the second is only
// ever set by somebody who already thought about it, and the people who need
// protecting are the ones who did not.

// aRoot is a directory tool arguments are confined to.
type aRoot struct {
	// given is what the operator asked for, kept for the refusal message so it
	// says the thing they typed rather than what it resolved to.
	given string
	// resolved is that path made absolute with every symlink followed, which is
	// what a candidate is actually compared against. Without it a link inside
	// the root is a way out of it.
	resolved string
}

// newRoot resolves the directory the server confines arguments to.
func newRoot(given string) (aRoot, error) {
	if given == "" {
		wd, err := os.Getwd()
		if err != nil {
			return aRoot{}, fmt.Errorf("no root was given and the working directory cannot be read: %w", err)
		}
		given = wd
	}
	resolved, err := resolve(given)
	if err != nil {
		return aRoot{}, fmt.Errorf("root %s: %w", given, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return aRoot{}, fmt.Errorf("root %s: %w", given, err)
	}
	if !info.IsDir() {
		return aRoot{}, fmt.Errorf("root %s is not a directory", given)
	}
	return aRoot{given: given, resolved: resolved}, nil
}

// confine refuses a request that names a path outside the root.
//
// Every path the request will touch is checked, not only the ones it reads: an
// output path is the one that overwrites something.
func (r aRoot) confine(req command.Request) error {
	for _, p := range req.Paths() {
		if p == "" {
			continue // an optional argument the caller left out
		}
		if err := r.holds(p); err != nil {
			return err
		}
	}
	return nil
}

// holds reports whether one path lies inside the root.
func (r aRoot) holds(p string) error {
	resolved, err := resolve(p)
	if err != nil {
		return fmt.Errorf("%s: %w", p, err)
	}
	// Compared by walking from the root rather than by string prefix. A prefix
	// test says "/" contains nothing, because appending a separator to it gives
	// "//", and it says /srv/data-old is inside /srv/data.
	rel, err := filepath.Rel(r.resolved, resolved)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil
	}
	return fmt.Errorf("%s is outside %s, which is the root this server was started with; "+
		"restart it with -root to widen that", p, r.given)
}

// resolve makes a path absolute and follows every symlink in it.
//
// A path that does not exist yet is the ordinary case for an output, so the
// deepest part of it that does exist is resolved and the rest appended. Only
// resolving paths that exist would leave a link pointing out of the root as a
// way to create a file beyond it.
func resolve(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	missing := ""
	for current := abs; ; {
		real, err := filepath.EvalSymlinks(current)
		if err == nil {
			return filepath.Join(real, missing), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			// Walked to the root of the filesystem without finding anything
			// that exists, which cannot happen for an absolute path but is not
			// worth looping forever over if it does.
			return abs, nil
		}
		missing = filepath.Join(filepath.Base(current), missing)
		current = parent
	}
}
