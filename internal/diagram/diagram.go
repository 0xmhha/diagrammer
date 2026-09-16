// Package diagram holds the Go form of a stage-3 diagram source.
//
// These types mirror schemas/diagram.schema.json, which is the contract. They
// are checked against it by TestSchemaBinding; when the two disagree the schema
// is right and this file is what gets corrected.
//
// A document here is laid out but not drawn. Boxes carry the cell they sit in
// rather than a pixel position, and connections carry their endpoints rather
// than a route: where a box sits is a layout decision, while how large it is
// and which way a line bends are rendering decisions that belong to stage 4.
package diagram

// Family names the diagram family a document holds.
//
// One document holds one family. The list grows as each family's composer
// lands, so a family absent here is one that is not built yet.
type Family string

const (
	FamilyComponent Family = "component"
)

// RelationshipKind separates what the model stated from what compose derived.
type RelationshipKind string

const (
	// KindDependency is stated by the model outright.
	KindDependency RelationshipKind = "dependency"
	// KindAssembly is derived: one component requires an interface another
	// provides, which UML draws as the two halves of a connector meeting.
	KindAssembly RelationshipKind = "assembly"
)

// DropReason says why a level could not draw a relationship.
type DropReason string

const (
	// ReasonSelfReference is both ends rolling up to the same box on this
	// level, where drawing it would be a loop that says nothing. The
	// relationship is drawn on the level below, where the ends are separate.
	ReasonSelfReference DropReason = "selfReference"
)

// PortKind is which side of an interface a port sits on.
type PortKind string

const (
	PortProvided PortKind = "provided"
	PortRequired PortKind = "required"
)

// Document is one family's diagram source.
type Document struct {
	SchemaVersion int        `json:"schemaVersion"`
	Family        Family     `json:"family"`
	Meta          Meta       `json:"meta"`
	Provenance    Provenance `json:"provenance"`
	// Levels holds every page, sorted by id. The one with no parent is the
	// overview; the rest are drilled into from a box above them.
	Levels     []Level    `json:"levels"`
	Accounting Accounting `json:"accounting"`
}

// Meta titles the diagram.
type Meta struct {
	Title    string `json:"title"`
	Subtitle string `json:"subtitle,omitempty"`
}

// Provenance names the stage-2 model this document was composed from, which is
// what compose actually read.
type Provenance struct {
	Model       string `json:"model"`
	GeneratedBy string `json:"generatedBy,omitempty"`
}

// Accounting is the relationships a scope was responsible for.
//
// Drawn plus Dropped equals Proven exactly, on the document and on every level
// within it. A relationship that is not drawn has to be somewhere a reader can
// find it, or the diagram quietly lies about what it shows.
type Accounting struct {
	// Proven is what the model asserted for this scope, after rolling each
	// relationship up to the boxes standing for its ends.
	Proven  int `json:"proven"`
	Drawn   int `json:"drawn"`
	Dropped int `json:"dropped"`
}

// Level is one page of the diagram.
type Level struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// Parent is the level this one is reached from, and is empty on the
	// overview.
	Parent string `json:"parent,omitempty"`
	// OpensFrom is the box on the parent level that opens this one. It is set
	// exactly when Parent is.
	OpensFrom string `json:"opensFrom,omitempty"`
	Grid      Grid   `json:"grid"`
	// Boxes is sorted by id. Every box sits in a distinct cell.
	Boxes []Box `json:"boxes"`
	// Regions frame the generation an unfolded level skipped.
	Regions []Region `json:"regions,omitempty"`
	// Connections is sorted by id, and carries endpoints only.
	Connections []Connection `json:"connections"`
	Accounting  Accounting   `json:"accounting"`
}

// Grid is how many cells a level's layout spans.
type Grid struct {
	Rows int `json:"rows"`
	Cols int `json:"cols"`
}

// Box is one component as it appears on one level.
type Box struct {
	// ID is the component's own id, so a box traces back to the model.
	ID         string `json:"id"`
	Label      string `json:"label"`
	Stereotype string `json:"stereotype,omitempty"`
	Row        int    `json:"row"`
	Col        int    `json:"col"`
	// Opens is the level reached by drilling into this box, when it has one.
	Opens string `json:"opens,omitempty"`
	// Region is the band this box belongs to, when the level has regions.
	Region string `json:"region,omitempty"`
	// Ports are carried through so stage 4 can draw them on the box edge.
	Ports []Port `json:"ports,omitempty"`
	// Dropped records every relationship belonging to this box that the level
	// could not draw. It is what the accounting invariant is checked against,
	// and the reason no relationship goes missing without leaving a trace.
	Dropped []DroppedRelationship `json:"dropped,omitempty"`
}

// Port is one interface a component provides or requires.
type Port struct {
	ID        string   `json:"id"`
	Name      string   `json:"name,omitempty"`
	Kind      PortKind `json:"kind"`
	Interface string   `json:"interface"`
}

// DroppedRelationship is one relationship a level could not draw.
type DroppedRelationship struct {
	// To is the other end, named by the box it rolled up to.
	To     string           `json:"to"`
	Kind   RelationshipKind `json:"kind"`
	Reason DropReason       `json:"reason"`
}

// Region frames a generation an unfolded level skipped, so the page reads as a
// structure rather than a flat list.
type Region struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Stereotype string `json:"stereotype,omitempty"`
}

// Connection is one relationship drawn on one level.
type Connection struct {
	ID    string           `json:"id"`
	From  string           `json:"from"`
	To    string           `json:"to"`
	Kind  RelationshipKind `json:"kind"`
	Label string           `json:"label,omitempty"`
	// Interface is what an assembly connector passes through, so stage 4 can
	// name the socket it draws.
	Interface string `json:"interface,omitempty"`
}
