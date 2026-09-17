package vcs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// Revision is the commit a source tree was checked out at.
//
// It is the wire type for all three stages: the graph records what it read, the
// stage-2 model repeats it, and the composed document carries it to the page.
// One type rather than three so that a hash cannot be described one way in the
// graph and another in the drawing made from it.
type Revision struct {
	// Commit is the object name HEAD resolved to.
	Commit string `json:"commit"`
	// Ref is the branch HEAD pointed at, and is empty when HEAD is detached.
	Ref string `json:"ref,omitempty"`
}

const (
	gitEntry     = ".git"
	headFile     = "HEAD"
	packedRefs   = "packed-refs"
	symrefPrefix = "ref: "
	// gitdirPrefix introduces the real directory in a .git file, which is what
	// a linked worktree and a submodule have in place of a directory.
	gitdirPrefix = "gitdir: "
	// refPrefix is the only namespace a HEAD is followed into. See refPath.
	refPrefix = "refs/"
)

// objectName matches a full object name in either hash a repository may use:
// forty hex digits for SHA-1, sixty-four for SHA-256.
var objectName = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)

// Of returns the revision of the checkout that dir sits in, and whether there
// was one to return.
//
// A false second result is an ordinary answer rather than a failure: a tree
// unpacked from an archive is in no checkout, and a repository before its first
// commit is at no commit. Either way the honest document says nothing, so
// neither is reported as an error.
//
// An error means something that is there could not be read, which is worth
// reporting because the answer is then unknown rather than absent.
func Of(dir string) (Revision, bool, error) {
	root, err := filepath.Abs(dir)
	if err != nil {
		return Revision{}, false, fmt.Errorf("resolve %s: %w", dir, err)
	}
	git, ok := findGitDir(root)
	if !ok {
		return Revision{}, false, nil
	}
	return headOf(git)
}

// findGitDir walks up from start looking for the directory git keeps its refs
// in, so that a graph of one package inside a repository still knows which
// commit that repository is at.
func findGitDir(start string) (string, bool) {
	for dir := start; ; {
		candidate := filepath.Join(dir, gitEntry)
		if info, err := os.Stat(candidate); err == nil {
			if info.IsDir() {
				return candidate, true
			}
			if resolved, ok := gitDirFromFile(candidate); ok {
				return resolved, true
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// gitDirFromFile reads the directory named by a .git file, which is what a
// linked worktree and a submodule have in place of a directory.
//
// The path comes out of a file in the tree being analysed rather than from the
// caller, which is the one place in this program that is true. What bounds it
// is that it has to turn out to be a git directory: the name is followed only
// as far as asking whether there is a HEAD in it, and a directory with no HEAD
// is refused before anything else is read. That leaves a caller pointed at a
// hostile repository able to learn whether some directory holds a file called
// HEAD, and nothing else; every path this then reads is under a directory that
// answered to that.
//
// Refusing to follow it at all was the alternative. It was not taken because it
// would report no revision for every worktree and every submodule, which are
// ordinary ways to have a checkout, and a wrong answer of "none" is still a
// wrong answer.
func gitDirFromFile(file string) (string, bool) {
	raw, err := os.ReadFile(file)
	if err != nil {
		return "", false
	}
	named, ok := strings.CutPrefix(strings.TrimSpace(string(raw)), gitdirPrefix)
	if !ok || named == "" {
		return "", false
	}
	if !filepath.IsAbs(named) {
		named = filepath.Join(filepath.Dir(file), named)
	}
	// #nosec G703 -- the taint is real and is answered above: the path is
	// accepted only if it is a directory holding a HEAD, and nothing outside
	// such a directory is ever read.
	if info, err := os.Stat(filepath.Join(named, headFile)); err != nil || info.IsDir() {
		return "", false
	}
	return named, true
}

// headOf resolves HEAD, which is either an object name or a pointer to a ref.
func headOf(git string) (Revision, bool, error) {
	raw, err := os.ReadFile(filepath.Join(git, headFile))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Revision{}, false, nil
		}
		return Revision{}, false, fmt.Errorf("read HEAD: %w", err)
	}
	head := strings.TrimSpace(string(raw))

	ref, symbolic := strings.CutPrefix(head, symrefPrefix)
	if !symbolic {
		// A detached HEAD holds the object name itself. Anything else is
		// something this does not understand, and saying nothing is the answer
		// to that.
		if !objectName.MatchString(head) {
			return Revision{}, false, nil
		}
		return Revision{Commit: head}, true, nil
	}

	ref = strings.TrimSpace(ref)
	commit, err := resolve(git, ref)
	if err != nil {
		return Revision{}, false, err
	}
	// A branch with no commit on it yet resolves to nothing, which is what a
	// repository looks like before its first one.
	if commit == "" {
		return Revision{}, false, nil
	}
	return Revision{Commit: commit, Ref: ref}, true, nil
}

// resolve turns a ref into an object name, from its own file or from the file
// git packs refs into once there are enough of them to be worth packing.
func resolve(git, ref string) (string, error) {
	loose, ok := refPath(git, ref)
	if !ok {
		return "", nil
	}
	raw, err := os.ReadFile(loose)
	switch {
	case err == nil:
		name := strings.TrimSpace(string(raw))
		if objectName.MatchString(name) {
			return name, nil
		}
		return "", nil
	case !errors.Is(err, fs.ErrNotExist):
		return "", fmt.Errorf("read ref %s: %w", ref, err)
	}
	return fromPackedRefs(git, ref)
}

// refPath is where ref's own file would be, and refuses a ref that would lead
// anywhere else.
//
// HEAD is a file inside the tree being analysed, so what it says is input
// rather than fact. A HEAD reading `ref: ../../../../etc/passwd` would
// otherwise be followed, and whatever came back would be reported as the commit
// a drawing was made from.
func refPath(git, ref string) (string, bool) {
	if !strings.HasPrefix(ref, refPrefix) {
		return "", false
	}
	if ref != path.Clean(ref) || strings.Contains(ref, "..") {
		return "", false
	}
	return filepath.Join(git, filepath.FromSlash(ref)), true
}

// fromPackedRefs finds ref in the packed-refs file.
//
// The format is one ref per line, object name first. A line opening with # is
// the header and one opening with ^ is the object an annotated tag points at,
// which is not what a ref resolves to.
func fromPackedRefs(git, ref string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(git, packedRefs))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("read packed-refs: %w", err)
	}
	for line := range strings.SplitSeq(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "^") {
			continue
		}
		name, named, ok := strings.Cut(line, " ")
		if !ok || strings.TrimSpace(named) != ref {
			continue
		}
		if objectName.MatchString(name) {
			return name, nil
		}
	}
	return "", nil
}
