package main

import (
	"fmt"
	"strings"
)

// slot is where a position sits in the tableau grid.
type slot struct{ row, col int }

// layouts place each position of a spread on a grid. The Celtic cross keeps its
// traditional shape: the cross on the left, the staff climbing on the right.
//
//	     [5]          [10]
//	[3] [1][2] [4]    [9]
//	     [6]           [8]
//	                   [7]
//
// A three card spread is simply a row.
var layouts = map[string][]slot{
	"celtic": {
		{1, 1}, // 1 the present, at the centre
		{1, 2}, // 2 crossing it
		{1, 0}, // 3 to the left
		{1, 3}, // 4 to the right
		{0, 1}, // 5 above
		{2, 1}, // 6 below
		{3, 4}, // 7 the foot of the staff
		{2, 4}, // 8
		{1, 4}, // 9
		{0, 4}, // 10 the head of the staff
	},
	"three": {{0, 0}, {0, 1}, {0, 2}},
}

const (
	tableauGap = 2
	minCellArt = 4
	maxCellArt = 14
)

// hasLayout reports whether a spread can be laid out as a tableau.
func hasLayout(s *Spread) bool {
	if s == nil {
		return false
	}
	l, ok := layouts[s.Key]
	return ok && len(l) >= len(s.Positions)
}

// tableau draws the whole reading at once, every card pictured in its place.
func tableau(r *Reading, opts options) string {
	if !hasLayout(r.Spread) {
		return " That reading has no layout to show.\n"
	}
	places := layouts[r.Spread.Key]

	rows, cols := 0, 0
	for i := range r.Positions {
		rows = max(rows, places[i].row+1)
		cols = max(cols, places[i].col+1)
	}

	height := cellHeight(opts)
	cells := make(map[slot][]string, len(r.Positions))
	width := 0
	for i, p := range r.Positions {
		cell := cellFor(i, p, height, opts)
		cells[places[i]] = cell
		width = max(width, columnWidth(cell))
	}

	var sb strings.Builder
	sb.WriteString(header(r))
	for row := range rows {
		// Every column takes the same width whether or not a card sits there,
		// so the cross keeps its shape around the empty middle.
		height := 0
		for col := range cols {
			height = max(height, len(cells[slot{row, col}]))
		}
		band := make([]string, height)
		for col := range cols {
			cell := cells[slot{row, col}]
			for i := range band {
				var seg string
				if i < len(cell) {
					seg = cell[i]
				}
				if col > 0 {
					band[i] += strings.Repeat(" ", tableauGap)
				}
				band[i] += seg + strings.Repeat(" ", max(width-visibleWidth(seg), 0))
			}
		}
		for i, l := range band {
			band[i] = strings.TrimRight(l, " ")
		}
		sb.WriteString(indent(strings.Join(band, "\n"), " ") + "\n \n")
	}
	sb.WriteString(divider + "\n")
	return sb.String()
}

// cellHeight picks the picture size for a tableau. Every spread uses the size a
// three card row would take, so a card is the same size in the Celtic cross as
// it is in a three card reading; a cross that will not fit is scrolled instead
// of shrunk.
func cellHeight(opts options) int {
	screenCols, screenRows := opts.cols, opts.rows
	if screenCols <= 0 {
		screenCols = defaultCols
	}
	if screenRows <= 0 {
		screenRows = 24
	}
	const acrossThree = 3
	// A card picture is about 1.14 columns wide per row of height.
	byWidth := int(float64((screenCols-tableauGap*(acrossThree-1)-2)/acrossThree) / 1.14)
	// One band of cards, its caption, and the reading's banner.
	byHeight := screenRows - 12
	return min(max(min(byWidth, byHeight), minCellArt), maxCellArt)
}

// cellFor is one card of the tableau: its picture, numbered underneath.
func cellFor(i int, p Position, height int, opts options) []string {
	art := photoLines(p, options{art: artPhoto, height: height, color: opts.color})
	if len(art) == 0 {
		art = sketchLines(p, options{art: artSketch})
	}
	width := max(columnWidth(art), 8)
	caption := fmt.Sprintf("%d. %s", i+1, p.Card.Name)
	if p.Card.Reversed {
		caption += " (rev)"
	}
	// Card names outrun a narrow cell, so let the caption take a second line
	// rather than truncating it.
	return append(art, strings.Split(wrap(caption, width), "\n")...)
}
