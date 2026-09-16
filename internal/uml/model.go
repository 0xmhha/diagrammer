// Package uml holds the Go form of the stage-2 UML codegraph.
//
// These types mirror schemas/codegraph.schema.json, which is the contract. They
// are checked against it by TestSchemaBinding; when the two disagree the schema
// is right and this file is what gets corrected.
//
// Nothing here is placed. A Model says what the diagram means; where anything
// goes is stage 3's decision.
package uml

// Family names one diagram family.
type Family string

const (
	FamilyComponent Family = "component"
	FamilySequence  Family = "sequence"
	FamilyState     Family = "state"
	FamilyUsecase   Family = "usecase"
)

// Families lists every family in the order they appear in the schema enum.
func Families() []Family {
	return []Family{FamilyComponent, FamilySequence, FamilyState, FamilyUsecase}
}

// Origin records how a Model came to exist.
//
// A fixture written by hand and one a model produced are different kinds of
// evidence, and a later reader cannot otherwise tell them apart.
type Origin string

const (
	// OriginHandwritten was authored by a person.
	OriginHandwritten Origin = "handwritten"
	// OriginModel was emitted by an LLM and not reviewed.
	OriginModel Origin = "model"
	// OriginModelReviewed was emitted by an LLM and checked by a person, which
	// is what a committed fixture should normally be.
	OriginModelReviewed Origin = "modelReviewed"
)

// Model is a UML model of a code graph, returned by a plugin's skill.
type Model struct {
	SchemaVersion int        `json:"schemaVersion"`
	Meta          Meta       `json:"meta"`
	Provenance    Provenance `json:"provenance"`
	// Families is what this model can support. A family is listed only when its
	// section is present and carries content; Validate enforces that agreement,
	// so a model claiming sequence support without lifelines is refused at the
	// boundary rather than three stages later.
	Families  []Family        `json:"families"`
	Component *ComponentModel `json:"component,omitempty"`
	Sequence  *SequenceModel  `json:"sequence,omitempty"`
	State     *StateModel     `json:"state,omitempty"`
	Usecase   *UsecaseModel   `json:"usecase,omitempty"`
}

// Meta titles the diagrams derived from a Model.
type Meta struct {
	Title    string `json:"title"`
	Subtitle string `json:"subtitle,omitempty"`
}

// Provenance records how a Model was produced.
type Provenance struct {
	Origin Origin `json:"origin"`
	Note   string `json:"note,omitempty"`
	// SourceGraph names the stage-1 graph this model was derived from, if one
	// was used.
	SourceGraph *SourceGraph `json:"sourceGraph,omitempty"`
}

// SourceGraph identifies a stage-1 graph.
type SourceGraph struct {
	Path   string `json:"path,omitempty"`
	SHA256 string `json:"sha256,omitempty"`
}

// --- component ---------------------------------------------------------------

// PortKind is which side of an interface a port sits on.
type PortKind string

const (
	PortProvided PortKind = "provided"
	PortRequired PortKind = "required"
)

// DependencyKind is the UML dependency stereotype.
type DependencyKind string

const (
	DependencyUse        DependencyKind = "use"
	DependencyRealize    DependencyKind = "realize"
	DependencySubstitute DependencyKind = "substitute"
)

// ComponentModel is UML component diagram vocabulary.
type ComponentModel struct {
	Components   []Component  `json:"components"`
	Interfaces   []Interface  `json:"interfaces,omitempty"`
	Dependencies []Dependency `json:"dependencies,omitempty"`
}

// Component is one deployable or logical part.
type Component struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Stereotype is written without guillemets, such as service or subsystem.
	Stereotype  string `json:"stereotype,omitempty"`
	Description string `json:"description,omitempty"`
	// Parent is the component this one is nested inside, and is empty at the
	// top level.
	//
	// The nesting is what stage 3 turns into levels and drill-down pages.
	// Without it a component diagram is one flat page, and a model of any size
	// is unreadable. Choosing the decomposition is the model's job, because it
	// is a judgement about meaning rather than a fact about the code.
	Parent string `json:"parent,omitempty"`
	// Ports carry both directions: the provided interfaces are the ports of
	// kind provided and the required ones are the rest. There is no second way
	// to say the same thing.
	Ports []Port `json:"ports,omitempty"`
}

// Port is where a component meets an interface.
type Port struct {
	ID   string   `json:"id"`
	Name string   `json:"name,omitempty"`
	Kind PortKind `json:"kind"`
	// Interface names an entry in ComponentModel.Interfaces.
	Interface string `json:"interface"`
}

// Interface is a named set of operations.
type Interface struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Operations []Operation `json:"operations,omitempty"`
}

// Operation is one entry on an interface.
type Operation struct {
	Name      string `json:"name"`
	Signature string `json:"signature,omitempty"`
}

// Dependency is one component or interface relying on another.
type Dependency struct {
	ID   string         `json:"id"`
	From string         `json:"from"`
	To   string         `json:"to"`
	Kind DependencyKind `json:"kind,omitempty"`
	Name string         `json:"name,omitempty"`
}

// --- sequence ----------------------------------------------------------------

// MessageKind is how one lifeline reaches another.
type MessageKind string

const (
	// MessageSync blocks the sender until a reply arrives.
	MessageSync MessageKind = "sync"
	// MessageAsync does not block the sender.
	MessageAsync MessageKind = "async"
	// MessageReply returns from a synchronous call.
	MessageReply MessageKind = "reply"
	// MessageCreate brings a lifeline into existence.
	MessageCreate MessageKind = "create"
	// MessageDestroy ends a lifeline.
	MessageDestroy MessageKind = "destroy"
)

// FragmentKind is the interaction operator of a combined fragment.
type FragmentKind string

const (
	FragmentAlt      FragmentKind = "alt"
	FragmentOpt      FragmentKind = "opt"
	FragmentLoop     FragmentKind = "loop"
	FragmentPar      FragmentKind = "par"
	FragmentCritical FragmentKind = "critical"
	FragmentBreak    FragmentKind = "break"
	FragmentStrict   FragmentKind = "strict"
	FragmentSeq      FragmentKind = "seq"
)

// SequenceModel is UML sequence diagram vocabulary.
//
// Message order is the order of Messages. There is no separate ordering field,
// because a second way to express order is a second thing that can disagree.
type SequenceModel struct {
	Lifelines   []Lifeline   `json:"lifelines"`
	Messages    []Message    `json:"messages"`
	Activations []Activation `json:"activations,omitempty"`
	Fragments   []Fragment   `json:"fragments,omitempty"`
}

// Lifeline is one participant.
type Lifeline struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Represents names a Component, when the model also carries that family.
	Represents string `json:"represents,omitempty"`
	// Actor draws a stick figure rather than a box, for the participant outside
	// the system.
	Actor bool `json:"actor,omitempty"`
}

// Message is one interaction between lifelines.
type Message struct {
	ID   string      `json:"id"`
	From string      `json:"from"`
	To   string      `json:"to"`
	Name string      `json:"name"`
	Kind MessageKind `json:"kind"`
	Note string      `json:"note,omitempty"`
}

// Activation is one execution on a lifeline, spanning the messages that start
// and end it.
type Activation struct {
	ID       string `json:"id"`
	Lifeline string `json:"lifeline"`
	Start    string `json:"start"`
	End      string `json:"end"`
}

// Fragment is a combined fragment over some of the messages.
type Fragment struct {
	ID   string       `json:"id"`
	Kind FragmentKind `json:"kind"`
	// Operands has one entry per branch for alt, and exactly one for opt, loop,
	// critical and break.
	Operands []Operand `json:"operands"`
}

// Operand is one branch of a fragment.
type Operand struct {
	Guard    string   `json:"guard,omitempty"`
	Messages []string `json:"messages"`
}

// --- state -------------------------------------------------------------------

// StateKind separates real states from pseudostates.
type StateKind string

const (
	StateSimple    StateKind = "simple"
	StateComposite StateKind = "composite"
	StateInitial   StateKind = "initial"
	StateFinal     StateKind = "final"
	StateChoice    StateKind = "choice"
	StateJunction  StateKind = "junction"
	StateHistory   StateKind = "history"
)

// StateModel is UML state machine vocabulary.
type StateModel struct {
	States      []State      `json:"states"`
	Transitions []Transition `json:"transitions"`
}

// State is one state or pseudostate.
type State struct {
	ID   string    `json:"id"`
	Name string    `json:"name"`
	Kind StateKind `json:"kind"`
	// Parent names a State of kind composite, and is empty at the top level.
	Parent   string `json:"parent,omitempty"`
	Entry    string `json:"entry,omitempty"`
	Exit     string `json:"exit,omitempty"`
	Activity string `json:"activity,omitempty"`
}

// Transition reads as trigger [guard] / effect.
//
// All three are optional: a transition with none is taken as soon as its source
// state completes.
type Transition struct {
	ID      string `json:"id"`
	From    string `json:"from"`
	To      string `json:"to"`
	Trigger string `json:"trigger,omitempty"`
	Guard   string `json:"guard,omitempty"`
	Effect  string `json:"effect,omitempty"`
}

// --- use case ----------------------------------------------------------------

// ActorKind separates the actor that starts a use case from one it calls on.
type ActorKind string

const (
	ActorPrimary   ActorKind = "primary"
	ActorSecondary ActorKind = "secondary"
)

// UsecaseModel is UML use case vocabulary.
type UsecaseModel struct {
	// System is the boundary. Every use case listed sits inside it and every
	// actor sits outside it; that placement is the model's claim and stage 4
	// must honour it.
	System       System        `json:"system"`
	Actors       []Actor       `json:"actors"`
	Usecases     []Usecase     `json:"usecases"`
	Associations []Association `json:"associations"`
	Includes     []Include     `json:"includes,omitempty"`
	Extends      []Extend      `json:"extends,omitempty"`
}

// System is the boundary a use case diagram draws.
type System struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Actor is a role outside the system.
type Actor struct {
	ID   string    `json:"id"`
	Name string    `json:"name"`
	Kind ActorKind `json:"kind,omitempty"`
}

// Usecase is one goal the system serves.
type Usecase struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// Association ties an actor to a use case.
type Association struct {
	ID      string `json:"id"`
	Actor   string `json:"actor"`
	Usecase string `json:"usecase"`
}

// Include is From always performing To.
type Include struct {
	ID   string `json:"id"`
	From string `json:"from"`
	To   string `json:"to"`
}

// Extend is From optionally adding behaviour to To.
//
// The arrow points from the extending use case to the extended one, which is
// the direction UML defines and the one most often drawn backwards.
type Extend struct {
	ID             string `json:"id"`
	From           string `json:"from"`
	To             string `json:"to"`
	ExtensionPoint string `json:"extensionPoint,omitempty"`
	Condition      string `json:"condition,omitempty"`
}
