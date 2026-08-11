package main

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	wrapWidth     = 65 // widest the prose ever wraps
	minTextWidth  = 34 // narrower than this and the words go under the picture
	defaultCols   = 80
	maxArtHeight  = 24 // the picture never grows past this, however tall the terminal
	cardArtHeight = 10 // -card's thumbnail: big enough to read, small enough to sit on a few lines
	artGap        = 2  // columns between the picture and the words
)

const divider = " *****************************************************************"

// artStyle selects how a card is pictured.
type artStyle string

const (
	artBoth   artStyle = "both"   // the deck scan and the line drawing, side by side
	artPhoto  artStyle = "photo"  // the deck scan, rendered as colored ASCII
	artSketch artStyle = "sketch" // the line drawing from the card guide
	artNone   artStyle = "none"
)

// wrap breaks text at spaces so no line exceeds width, like `fold -w N -s`.
func wrap(text string, width int) string {
	var out []string
	for _, para := range strings.Split(text, "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			out = append(out, "")
			continue
		}
		line := words[0]
		for _, w := range words[1:] {
			if len(line)+1+len(w) > width {
				out = append(out, line)
				line = w
				continue
			}
			line += " " + w
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// indent puts prefix in front of every line.
func indent(text, prefix string) string {
	lines := strings.Split(text, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}

// block renders a labelled paragraph, or nothing if the text is empty.
func block(label, text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	if label != "" {
		text = label + " " + text
	}
	return indent(wrap(text, wrapWidth), " ") + "\n \n"
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// visibleWidth is the width a line occupies on screen, ignoring color codes.
func visibleWidth(s string) int {
	return len([]rune(ansiPattern.ReplaceAllString(s, "")))
}

// sideBySide lays the right block out to the right of the left one, both
// starting at the top.
func sideBySide(left, right []string, gap int) []string {
	if len(left) == 0 {
		return right
	}
	if len(right) == 0 {
		return left
	}
	width := 0
	for _, l := range left {
		if w := visibleWidth(l); w > width {
			width = w
		}
	}
	// Both columns start at the top: the drawing and the words beneath it read
	// as one column beside the picture.
	out := make([]string, max(len(left), len(right)))
	for i := range out {
		var l, r string
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		pad := width - visibleWidth(l) + gap
		out[i] = strings.TrimRight(l+strings.Repeat(" ", pad)+r, " ")
	}
	return out
}

// photoLines renders the deck scan, or nothing if the style leaves it out.
func photoLines(p Position, opts options) []string {
	if opts.art != artPhoto && opts.art != artBoth {
		return nil
	}
	rendered, err := renderCard(p.Card, opts.height, opts.color)
	if err != nil {
		return []string{fmt.Sprintf("(no image: %v)", err)}
	}
	return strings.Split(strings.TrimRight(rendered, "\n"), "\n")
}

// sketchLines is the guide's drawing, or nothing if the style leaves it out.
func sketchLines(p Position, opts options) []string {
	if opts.art != artSketch && opts.art != artBoth {
		return nil
	}
	s := sketch(p)
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// sketch is the line drawing from the guide, flipped for a reversed card.
func sketch(p Position) string {
	if p.Notes == nil || p.Notes.Sketch == "" {
		return ""
	}
	lines := strings.Split(p.Notes.Sketch, "\n")
	if p.Card.Reversed {
		for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
			lines[i], lines[j] = lines[j], lines[i]
		}
	}
	return strings.Join(lines, "\n")
}

// cardText is everything written about the card, wrapped to width: what is
// drawn on it, what the position it fell in signifies, and the guide's summary
// for this orientation. The card's name belongs to the heading, not here.
func cardText(p Position, opts options, width int) []string {
	var sb strings.Builder
	if p.Notes != nil {
		sb.WriteString(para(p.Notes.Imagery, width))
	}
	sb.WriteString(para(p.Meaning, width))

	orientation := "Upright: "
	if p.Card.Reversed {
		orientation = "Reversed: "
	}
	sb.WriteString(para(orientation+p.Notes.Summary(p.Card.Reversed), width))
	if opts.detail && p.Notes != nil {
		sb.WriteString(para("Waite (1911): "+p.Notes.Waite(p.Card.Reversed), width))
	}
	return strings.Split(strings.TrimRight(sb.String(), "\n"), "\n")
}

// para wraps a paragraph and leaves a blank line after it.
func para(text string, width int) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	return wrap(text, width) + "\n\n"
}

// fullHeight is the tallest a single card picture can be without spilling
// past the terminal in either direction — the z key's fullscreen view, so a
// card can fill the window on the spot instead of the reader having to widen
// their tmux pane to see any more of it.
func fullHeight(cols, rows int) int {
	if cols <= 0 {
		cols = defaultCols
	}
	if rows <= 0 {
		rows = 24
	}
	// A card picture is about 1.14 columns wide per row of height, the same
	// ratio cellHeight uses to size the tableau's grid.
	byWidth := int(float64(cols-2) / 1.14)
	byHeight := rows - 3 // the heading and the status bar
	return max(min(byWidth, byHeight), minCellArt)
}

// page lays a card out: the deck scan on the left, and the guide's drawing with
// the words beneath it on the right, so a whole card fits on one screen.
func page(p Position, opts options) string {
	if opts.full {
		// Fullscreen only ever grows the picture: a -height flag (or a
		// smaller terminal than the one the height was fit to) may already
		// call for something taller than fits in the window, and z should
		// not shrink that back down.
		opts.height = max(opts.height, fullHeight(opts.cols, opts.rows))
	}
	photo := photoLines(p, opts)
	cols := opts.cols
	if cols <= 0 {
		cols = defaultCols
	}

	// The words take whatever the picture leaves, within reason; if that is too
	// cramped they go underneath it instead.
	width := wrapWidth
	if len(photo) > 0 {
		width = min(cols-columnWidth(photo)-artGap-2, wrapWidth)
	}

	var lines []string
	switch {
	case opts.bare:
		// Just the picture, unnamed: something to sit with rather than read.
		lines = photo
	case width < minTextWidth:
		width = min(cols-2, wrapWidth)
		lines = append(lines, photo...)
		lines = append(lines, "")
		lines = append(lines, sketchLines(p, opts)...)
		lines = append(lines, "")
		lines = append(lines, cardText(p, opts, width)...)
	default:
		right := sketchLines(p, opts)
		if len(right) > 0 {
			right = append(right, "")
		}
		right = append(right, cardText(p, opts, width)...)
		lines = sideBySide(photo, right, artGap)
	}

	var sb strings.Builder
	if !opts.bare {
		fmt.Fprintf(&sb, "  %s\n", p.heading())
		if p.Label != "" {
			fmt.Fprintf(&sb, "  %s\n", p.Label)
		}
		sb.WriteString(" \n")
	}
	sb.WriteString(indent(strings.Join(lines, "\n"), " ") + "\n \n")
	if !opts.bare {
		// The divider is wider than a bare picture and wraps badly on a
		// narrow terminal, with nothing else on the card to make sense of it.
		sb.WriteString(divider + "\n")
	}
	return sb.String()
}

// columnWidth is the widest visible line in a block.
func columnWidth(lines []string) int {
	w := 0
	for _, l := range lines {
		w = max(w, visibleWidth(l))
	}
	return w
}

// detail is Waite's 1911 divinatory meaning for this orientation.
func detail(p Position) string {
	if p.Notes == nil {
		return ""
	}
	return block("Waite (1911):", p.Notes.Waite(p.Card.Reversed))
}

// mindful is the long contemplative essay, shown only when asked for.
func mindful(p Position) string {
	if p.Notes == nil || p.Notes.MindfulBody == "" {
		return " The guide has no mindful reading for this card.\n"
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "  %s — %s\n \n", p.title(), p.Notes.MindfulTitle)
	for _, para := range strings.Split(p.Notes.MindfulBody, "\n\n") {
		sb.WriteString(block("", para))
	}
	sb.WriteString(divider + "\n")
	return sb.String()
}

// header is the banner shown before a reading and at the top of exports.
func header(r *Reading) string {
	var sb strings.Builder
	sb.WriteString(divider + "\n")
	if r.Spread != nil {
		fmt.Fprintf(&sb, " %s — %s\n", r.Spread.Title, r.Spread.Blurb)
	}
	if r.Query == "" {
		sb.WriteString(" Your query was: (none)\n")
	} else {
		sb.WriteString(indent(wrap("Your query was: "+r.Query, wrapWidth), " ") + "\n")
	}
	sb.WriteString(divider + "\n \n")
	return sb.String()
}

// plain is the whole reading as text: the "-no-fancy" output.
func plain(r *Reading, opts options) string {
	var sb strings.Builder
	sb.WriteString(header(r))
	for _, p := range r.Positions {
		sb.WriteString(page(p, opts))
	}
	return sb.String()
}

// journalBlock is the reader's own note about the reading, wrapped like any
// other paragraph, or nothing if they never added one.
func journalBlock(r *Reading) string {
	return block("Journal:", r.Journal)
}

// exportBody is what goes to the exported file: the header, then whatever the
// reader wrote down about the reading, then the whole spread laid out (if it
// has a layout), then each card the way page() already shows it — more
// context than the interactive walkthrough gives on its own, since there is
// no keyboard in a file to open the tableau or type a note later.
func exportBody(r *Reading, opts options) string {
	var sb strings.Builder
	sb.WriteString(header(r))
	sb.WriteString(journalBlock(r))
	if hasLayout(r.Spread) {
		sb.WriteString(tableauGrid(r, opts))
	}
	for _, p := range r.Positions {
		sb.WriteString(page(p, opts))
	}
	return sb.String()
}
