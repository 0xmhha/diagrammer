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
