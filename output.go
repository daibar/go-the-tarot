package main

import (
	"fmt"
	"regexp"
	"strings"
)

const wrapWidth = 65

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

// sideBySide lays the right block out to the right of the left one, centering
// the shorter of the two vertically.
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
	// Whichever block is shorter gets padded out at the top.
	height := max(len(left), len(right))
	leftTop := (height - len(left)) / 2
	rightTop := (height - len(right)) / 2

	out := make([]string, height)
	for i := range out {
		var l, r string
		if i >= leftTop && i-leftTop < len(left) {
			l = left[i-leftTop]
		}
		if i >= rightTop && i-rightTop < len(right) {
			r = right[i-rightTop]
		}
		pad := width - visibleWidth(l) + gap
		out[i] = l + strings.Repeat(" ", pad) + r
	}
	return out
}

// art pictures the card in the configured style.
func art(p Position, opts options) string {
	var photo, drawing []string
	if opts.art == artPhoto || opts.art == artBoth {
		rendered, err := renderCard(p.Card, opts.height, opts.color)
		if err != nil {
			return fmt.Sprintf("  (no image: %v)\n", err)
		}
		photo = strings.Split(strings.TrimRight(rendered, "\n"), "\n")
	}
	if opts.art == artSketch || opts.art == artBoth {
		if s := sketch(p); s != "" {
			drawing = strings.Split(s, "\n")
		}
	}
	lines := sideBySide(photo, drawing, 3)
	if len(lines) == 0 {
		return ""
	}
	return indent(strings.Join(lines, "\n"), " ") + "\n \n"
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

// page is the full display for one card: the position it fell in, the picture,
// what is drawn on it, its name, and the guide's summary for this orientation.
func page(p Position, opts options) string {
	var sb strings.Builder
	if p.Name != "" {
		fmt.Fprintf(&sb, "  %s\n \n", p.Name)
	}
	sb.WriteString(art(p, opts))
	if p.Notes != nil {
		sb.WriteString(block("", p.Notes.Imagery))
	}
	fmt.Fprintf(&sb, "  %s\n \n", p.title())

	sb.WriteString(block("", p.Meaning))

	orientation := "Upright:"
	if p.Card.Reversed {
		orientation = "Reversed:"
	}
	sb.WriteString(block(orientation, p.Notes.Summary(p.Card.Reversed)))

	if opts.detail {
		sb.WriteString(detail(p))
	}
	sb.WriteString(divider + "\n")
	return sb.String()
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

// plain is the whole reading as text: the "no fancy" output and the export.
func plain(r *Reading, opts options) string {
	var sb strings.Builder
	sb.WriteString(header(r))
	for _, p := range r.Positions {
		sb.WriteString(page(p, opts))
	}
	return sb.String()
}
