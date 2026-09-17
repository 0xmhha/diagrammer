package artifact

// Point is a position in a page's own pixel space.
type Point struct{ X, Y float64 }

// Box is one drawn box.
//
// It carries its label and its drill-down target as well as its geometry,
// because both are in the artifact: the label is drawn and the target is the
// attribute the viewer follows. A reader parsing the document back gets what
// the document actually says, not a subset chosen for one caller.
type Box struct {
	ID    string
	Label string
	// Stereotype is what the model called this thing, carried so the drawing
	// can give it a shape a reader recognises: a final state is a ringed
	// circle, an actor a figure, a use case an ellipse.
	Stereotype string
	// Opens names the level reached by following this box, and is empty when
	// there is none.
	Opens string
	X, Y  float64
	W, H  float64
}

// Rect is a plain rectangle in the diagram's pixel space.
type Rect struct{ X, Y, W, H float64 }

func (r Rect) Right() float64  { return r.X + r.W }
func (r Rect) Bottom() float64 { return r.Y + r.H }

// Overlaps reports whether two rectangles share any area. Touching along an
// edge is not overlapping: two things drawn flush against each other are a
// layout decision, not a collision.
func (r Rect) Overlaps(o Rect) bool {
	return r.X < o.Right() && r.Right() > o.X && r.Y < o.Bottom() && r.Bottom() > o.Y
}

func (b Box) Right() float64  { return b.X + b.W }
func (b Box) Bottom() float64 { return b.Y + b.H }

// Region is a band drawn around boxes.
type Region struct {
	ID    string
	Label string
	X, Y  float64
	W, H  float64
}

func (r Region) Right() float64  { return r.X + r.W }
func (r Region) Bottom() float64 { return r.Y + r.H }

// Route is one drawn connection, with the polyline it follows.
//
// The points are what the composition rules read. They are written into the
// artifact unrounded, because the rules test distances against thresholds and a
// value sitting on a threshold can have its verdict flipped by rounding.
type Route struct {
	ID       string
	From     string
	To       string
	FromSide string
	ToSide   string
	Points   []Point
	// LabelAt is where the connection's text sits, and is the zero value when
	// it has none.
	LabelAt   Point
	LabelText string
	// LabelBounds is the rectangle the text actually occupies, written by the
	// renderer and read back rather than worked out again here.
	//
	// How wide a string is depends on the font the page asks for and on the
	// size the renderer settled on for this one label, neither of which a
	// reader of the artifact can see. A checker that guessed would be judging a
	// rectangle nobody drew, which is the same mistake as recomputing a label's
	// position instead of reading it.
	LabelBounds Rect
	// LabelSize is the size the text is written at. It is not always the same:
	// a label is shrunk to the room its run has before it is cut.
	LabelSize float64
	// LabelAnchor is how the text is hung off LabelAt: centred above a line
	// that runs across the page, or started beside one that runs down it.
	LabelAnchor string
}

// Bar is one execution drawn on a lifeline, in the sequence family.
type Bar struct {
	ID string
	// Box is the lifeline this execution runs on.
	Box  string
	X, Y float64
	W, H float64
}

func (b Bar) Bottom() float64 { return b.Y + b.H }

// Frame is one combined fragment drawn around part of a ladder.
type Frame struct {
	ID    string
	Kind  string
	Label string
	X, Y  float64
	W, H  float64
}

func (f Frame) Right() float64  { return f.X + f.W }
func (f Frame) Bottom() float64 { return f.Y + f.H }

// Scene is one page as drawn.
type Scene struct {
	// Family says which kind of diagram this is, because the rules a drawing is
	// held to differ by family and a scene travels without its document.
	Family  string
	Level   string
	Title   string
	Width   float64
	Height  float64
	Boxes   []Box
	Regions []Region
	Routes  []Route
	// Bars and Frames are the sequence family's own furniture, and are empty
	// for every other.
	Bars   []Bar
	Frames []Frame
}

// BoxByID finds a box on this page.
func (s *Scene) BoxByID(id string) (Box, bool) {
	for _, b := range s.Boxes {
		if b.ID == id {
			return b, true
		}
	}
	return Box{}, false
}
