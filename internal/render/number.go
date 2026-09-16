package render

import (
	"fmt"
	"math"
	"strconv"
)

// num formats a coordinate for the artifact.
//
// One helper, used everywhere a number reaches the output, so the document
// cannot carry two spellings of the same value. The shortest form that round
// trips is what is written: it is the least noise that loses nothing.
//
// Negative zero is normalised, because it is the same number as zero and
// writing it as "-0" would make two identical drawings differ by a character.
//
// NaN and infinity fail rather than serialise. A coordinate that is not a
// number is a defect upstream, and writing it out would produce a document that
// validates, draws as nothing, and gives no clue where the arithmetic went
// wrong.
func num(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		panic(fmt.Sprintf("render: refusing to write a coordinate that is not a number: %v", v))
	}
	if v == 0 {
		return "0"
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// jsRound rounds half away from... no: half up, the way a browser does.
//
// It exists because the renderer's arithmetic mirrors decisions first made in
// JavaScript, and the two languages disagree on negative halves: -2.5 rounds to
// -2 there and -3 with Go's math.Round. Where a value came from a mirrored
// expression, this is the rounding that keeps the shape the same.
func jsRound(v float64) float64 { return math.Floor(v + 0.5) }

// quantize trims a coordinate to one decimal.
//
// Routes are laid out on whole pixels, so a long decimal in the output means an
// intermediate value escaped rather than that the geometry needed the
// precision. Trimming keeps the artifact readable and the comparison stable.
func quantize(v float64) float64 { return jsRound(v*10) / 10 }
