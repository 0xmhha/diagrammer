// Package vcs reads what version control already knows about a source tree.
//
// It reads the files under .git rather than running git. This program starts no
// external process anywhere else, and a subprocess would make the answer depend
// on whether git is installed and on what a person's configuration does to it.
// The two or three files that hold a ref need neither.
//
// What it answers is deliberately narrow: which commit was checked out. It does
// not answer whether the working tree matched that commit, because deciding
// that means reading the object store and is a different order of work. It does
// not read the remote, the author, or anything else that would put a person or
// a machine into a document that travels.
package vcs
