// Package symlink is a fixture whose point is the file beside this one.
//
// linked.go is a symlink to a file outside this directory. Following it would
// pull that file's declarations into a graph of this tree and record them under
// an in-tree path, so nothing in the output would say where they came from.
package symlink

// Own belongs to this tree and must appear.
func Own() {}
