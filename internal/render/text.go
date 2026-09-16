package render

import (
	"strings"

	"golang.org/x/text/width"
)

// Text is measured by arithmetic rather than by asking a font, because a
// renderer that needed font metrics would need a font engine, and the
// difference does not pay for itself on a diagram of boxes.
const (
	// widthFactor is the advance width of one text unit, per pixel of font
	// size. It is the ratio a monospaced-ish sans face averages, and the labels
	// here are short enough that averaging is honest.
	widthFactor = 0.6
	// textPadding is the total horizontal space reserved inside a box so text
	// never touches the border.
	textPadding = 12
)

// textUnits counts how much horizontal room a string needs, in units of one
// narrow character.
//
// East Asian wide and fullwidth characters take two units, which is what the
// Unicode standard says of them and what a reader sees. The classification
// comes from the standard rather than from a hand-maintained table: this
// renderer is not imitating another one, so there is nothing to match except
// the characters themselves.
//
// A variation selector adds nothing of its own. It re-presents the character
// before it, and the presentation it asks for decides that character's width:
// the emoji form is wide, the text form narrow.
func textUnits(s string) int {
	units := 0
	runes := []rune(s)
	for i, r := range runes {
		if isVariationSelector(r) {
			continue
		}
		if i+1 < len(runes) {
			switch runes[i+1] {
			case variationSelectorEmoji:
				units += 2
				continue
			case variationSelectorText:
				units++
				continue
			}
		}
		units += runeUnits(r)
	}
	return units
}

const (
	variationSelectorFirst = 0xfe00
	variationSelectorLast  = 0xfe0f
	variationSelectorText  = 0xfe0e
	variationSelectorEmoji = 0xfe0f
)

func isVariationSelector(r rune) bool {
	return r >= variationSelectorFirst && r <= variationSelectorLast
}

func runeUnits(r rune) int {
	switch width.LookupRune(r).Kind() {
	case width.EastAsianWide, width.EastAsianFullwidth:
		return 2
	default:
		return 1
	}
}

// textWidth is the pixels a string occupies at a font size.
func textWidth(s string, fontSize float64) float64 {
	return float64(textUnits(s)) * fontSize * widthFactor
}

// fittedFontSize is the largest size at or below preferred that fits text
// inside available pixels, floored at minimum.
//
// Below the floor the text stops being legible, and shrinking further would
// hide a problem rather than solve it; the caller is expected to widen the box
// or report it.
func fittedFontSize(text string, available, preferred, minimum float64) float64 {
	units := textUnits(text)
	if units < 1 {
		units = 1
	}
	room := available - textPadding
	if room < 1 {
		room = 1
	}
	fitted := room / (float64(units) * widthFactor)
	if fitted > preferred {
		fitted = preferred
	}
	// Quantised to one decimal so the same label always yields the same size,
	// whatever rounding the arithmetic happened to land on.
	fitted = float64(int(fitted*10)) / 10
	if fitted < minimum {
		return minimum
	}
	return fitted
}

// truncate shortens a label to fit, ending with an ellipsis so a reader can see
// that something was cut rather than wondering whether the name is odd.
func truncate(s string, units int) string {
	if textUnits(s) <= units || units < 2 {
		return s
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		w := runeUnits(r)
		if used+w > units-1 {
			break
		}
		b.WriteRune(r)
		used += w
	}
	return b.String() + "…"
}
