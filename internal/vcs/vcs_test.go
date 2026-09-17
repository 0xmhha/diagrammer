package vcs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0xmhha/diagrammer/internal/vcs"
)

// The repositories here are built by writing the files git would have written,
// which is the point: this package claims to read a checkout without running
// git, and a test that ran git to make its fixtures would be testing that claim
// against a tool it says it does not need.

const (
	sha1Name   = "0123456789abcdef0123456789abcdef01234567"
	sha256Name = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

// repo writes a .git directory holding the files named, each relative to it.
func repo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		full := filepath.Join(root, ".git", filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatalf("make %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}
	return root
}

func TestOf(t *testing.T) {
	cases := []struct {
		name   string
		files  map[string]string
		commit string
		ref    string
	}{
		{
			name: "a branch with a loose ref",
			files: map[string]string{
				"HEAD":                 "ref: refs/heads/main\n",
				"refs/heads/main":      sha1Name + "\n",
				"refs/heads/elsewhere": "ffffffffffffffffffffffffffffffffffffffff\n",
			},
			commit: sha1Name,
			ref:    "refs/heads/main",
		},
		{
			// Once there are enough refs to be worth packing, the branch file
			// is gone and the answer is in packed-refs instead. A reader that
			// only knew about loose refs would report nothing on any repository
			// that had been gc'd, which is most of them.
			name: "a branch that has been packed away",
			files: map[string]string{
				"HEAD": "ref: refs/heads/main\n",
				"packed-refs": "# pack-refs with: peeled fully-peeled sorted \n" +
					"ffffffffffffffffffffffffffffffffffffffff refs/heads/other\n" +
					sha1Name + " refs/heads/main\n" +
					"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee refs/tags/v1\n" +
					"^dddddddddddddddddddddddddddddddddddddddd\n",
			},
			commit: sha1Name,
			ref:    "refs/heads/main",
		},
		{
			name:   "a detached head names the commit itself",
			files:  map[string]string{"HEAD": sha1Name + "\n"},
			commit: sha1Name,
		},
		{
			name: "a repository hashing with sha256",
			files: map[string]string{
				"HEAD":            "ref: refs/heads/main\n",
				"refs/heads/main": sha256Name + "\n",
			},
			commit: sha256Name,
			ref:    "refs/heads/main",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, known, err := vcs.Of(repo(t, c.files))
			if err != nil {
				t.Fatalf("Of: %v", err)
			}
			if !known {
				t.Fatal("want a revision, got none")
			}
			if got.Commit != c.commit {
				t.Errorf("commit is %q, want %q", got.Commit, c.commit)
			}
			if got.Ref != c.ref {
				t.Errorf("ref is %q, want %q", got.Ref, c.ref)
			}
		})
	}
}

// TestNothingIsSaidWhenNothingIsKnown covers every way the answer is honestly
// absent. Each of these would be a lie if it returned a commit, and a failure
// if it returned an error: none of them is a broken repository, and a tree that
// is not in one is the ordinary case for a program pointed at a directory.
func TestNothingIsSaidWhenNothingIsKnown(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		bare  bool
	}{
		{name: "a directory in no repository at all", bare: true},
		{
			name:  "a repository before its first commit",
			files: map[string]string{"HEAD": "ref: refs/heads/main\n"},
		},
		{
			name:  "a HEAD holding something that is not an object name",
			files: map[string]string{"HEAD": "not a commit\n"},
		},
		{
			name: "a ref holding something that is not an object name",
			files: map[string]string{
				"HEAD":            "ref: refs/heads/main\n",
				"refs/heads/main": "not a commit\n",
			},
		},
		{
			name: "an abbreviated object name, which is not one",
			files: map[string]string{
				"HEAD":            "ref: refs/heads/main\n",
				"refs/heads/main": "0123456\n",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			if !c.bare {
				dir = repo(t, c.files)
			}
			got, known, err := vcs.Of(dir)
			if err != nil {
				t.Fatalf("Of: %v", err)
			}
			if known {
				t.Errorf("want nothing said, got commit %q", got.Commit)
			}
		})
	}
}

// TestAHeadIsNotFollowedOutOfTheRepository is the one that matters for safety.
//
// HEAD is a file in the tree being analysed, so a repository handed to this
// program chooses what it says. A ref is followed by joining it to the git
// directory, and a ref that climbs out of it would have this reading a file of
// the repository's choosing and reporting whatever was in it as a commit.
func TestAHeadIsNotFollowedOutOfTheRepository(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(outside, []byte(sha1Name+"\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	for _, ref := range []string{
		"../../../../../../../../" + outside,
		"refs/../../../../../../../.." + outside,
		"refs/heads/../../../../../../../../etc/passwd",
		"/etc/passwd",
	} {
		t.Run(ref, func(t *testing.T) {
			got, known, err := vcs.Of(repo(t, map[string]string{"HEAD": "ref: " + ref + "\n"}))
			if err != nil {
				t.Fatalf("Of: %v", err)
			}
			if known {
				t.Errorf("a ref leading outside was followed and returned %q", got.Commit)
			}
		})
	}
}

// TestAGitFileIsFollowed covers a linked worktree and a submodule, where .git
// is a file naming the real directory. Both are ordinary ways to have a
// checkout, and a reader that only understood a directory would say nothing
// about either.
func TestAGitFileIsFollowed(t *testing.T) {
	elsewhere := t.TempDir()
	for name, body := range map[string]string{
		"HEAD":            "ref: refs/heads/work\n",
		"refs/heads/work": sha1Name + "\n",
	} {
		full := filepath.Join(elsewhere, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatalf("make %s: %v", filepath.Dir(full), err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			t.Fatalf("write %s: %v", full, err)
		}
	}

	work := t.TempDir()
	if err := os.WriteFile(filepath.Join(work, ".git"), []byte("gitdir: "+elsewhere+"\n"), 0o600); err != nil {
		t.Fatalf("write .git: %v", err)
	}

	got, known, err := vcs.Of(work)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	if !known || got.Commit != sha1Name {
		t.Fatalf("want %s through the .git file, got %+v", sha1Name, got)
	}
}

// TestASubdirectoryFindsTheRepositoryAboveIt is how this is actually used: the
// directory handed to graph is usually a package inside a repository rather
// than its root.
func TestASubdirectoryFindsTheRepositoryAboveIt(t *testing.T) {
	root := repo(t, map[string]string{
		"HEAD":            "ref: refs/heads/main\n",
		"refs/heads/main": sha1Name + "\n",
	})
	deep := filepath.Join(root, "internal", "render")
	if err := os.MkdirAll(deep, 0o700); err != nil {
		t.Fatalf("make %s: %v", deep, err)
	}

	got, known, err := vcs.Of(deep)
	if err != nil {
		t.Fatalf("Of: %v", err)
	}
	if !known || got.Commit != sha1Name {
		t.Fatalf("want the repository above, got %+v", got)
	}
}

// TestAGitFileIsOnlyFollowedToAGitDirectory is the bound on the one path in
// this package that comes out of a file rather than from the caller.
//
// A .git file in a hostile tree can name any directory on the machine. What
// stops that from being a way to have arbitrary files read is that the named
// directory has to hold a HEAD before anything under it is opened.
func TestAGitFileIsOnlyFollowedToAGitDirectory(t *testing.T) {
	elsewhere := t.TempDir()
	if err := os.WriteFile(filepath.Join(elsewhere, "secret"), []byte("not for you\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	for _, named := range []string{elsewhere, filepath.Join(elsewhere, "secret"), "/etc", "/"} {
		t.Run(named, func(t *testing.T) {
			work := t.TempDir()
			if err := os.WriteFile(filepath.Join(work, ".git"), []byte("gitdir: "+named+"\n"), 0o600); err != nil {
				t.Fatalf("write .git: %v", err)
			}
			got, known, err := vcs.Of(work)
			if err != nil {
				t.Fatalf("Of: %v", err)
			}
			if known {
				t.Errorf("a directory that is not a git directory was followed and returned %q", got.Commit)
			}
		})
	}
}
