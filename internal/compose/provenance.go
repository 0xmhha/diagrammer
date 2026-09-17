package compose

import (
	"github.com/0xmhha/diagrammer/internal/diagram"
	"github.com/0xmhha/diagrammer/internal/uml"
)

// provenanceOf records where a composed document came from: the model that was
// read, and the commit that model says its source was checked out at.
//
// The revision is repeated rather than looked up. compose reads a model and
// never sees a source tree, so the only revision it can honestly report is the
// one it was handed. A model carrying none produces a document claiming none,
// which is the right answer rather than a gap to fill in.
//
// It is one function rather than a line in each of the four composers so that a
// family cannot quietly stop carrying it.
func provenanceOf(source string, model *uml.Model) diagram.Provenance {
	out := diagram.Provenance{Model: source, GeneratedBy: generatedBy}
	if model.Provenance.SourceGraph != nil {
		out.Revision = model.Provenance.SourceGraph.Revision
	}
	return out
}
