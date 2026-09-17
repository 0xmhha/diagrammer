// Package instruct writes the instruction stage 2 is performed from.
//
// Stage 2 happens outside this binary: a plugin's skill reads a stage-1 graph
// and returns a UML model, and this program never calls a model itself. That
// leaves a gap nothing else fills. The skill has to be told what to return, and
// until this package existed the only way to learn it was to find the schema
// file in the source.
//
// The instruction is built rather than stored. Every claim it makes about the
// contract is read from the embedded schemas at the moment it is asked for, so
// the instruction given to a model and the check applied to what it returns
// come from one file. A stored copy would drift, and drift here is a model told
// to produce something the gate refuses.
package instruct
