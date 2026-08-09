package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// CardNotes is one card's entry from the tarot.md guide: a short summary for
// each orientation, plus the longer material shown on request.
type CardNotes struct {
	Name          string
	Label         string // the heading as written, e.g. "0. The Fool"
	Sketch        string // the ASCII drawing from the fenced block
	Imagery       string
	Upright       string
	Reversed      string
	MindfulTitle  string
	MindfulBody   string
	WaiteUpright  string
	WaiteReversed string
}

// Section is a group of cards in the guide: the Major Arcana, or one suit.
type Section struct {
	Title string
	Cards []*CardNotes
}

// Guide is the parsed card guide: every card, and the outline they sit in.
type Guide struct {
	ByName   map[string]*CardNotes
	Sections []Section
}

// Summary is the orientation-appropriate guidance shown with every card.
func (n *CardNotes) Summary(reversed bool) string {
	if n == nil {
		return ""
	}
	if reversed {
		return n.Reversed
	}
	return n.Upright
}

// Waite is the 1911 divinatory meaning for this orientation.
func (n *CardNotes) Waite(reversed bool) string {
	if n == nil {
		return ""
	}
	if reversed {
		return n.WaiteReversed
	}
	return n.WaiteUpright
}

var (
	majorHeading  = regexp.MustCompile(`^### (\d+\. (.+))$`)
	minorHeading  = regexp.MustCompile(`^#### (.+)$`)
	suitHeading   = regexp.MustCompile(`^### (Suit of .+)$`)
	arcanaHeading = regexp.MustCompile(`^## (.*Arcana.*)$`)
	waiteLine     = regexp.MustCompile(`^> \*\*Waite \(1911\) — (Upright|Reversed):\*\*\s*(.*)$`)
)

// notesNameFixups reconciles spellings between the guide and the deck data.
var notesNameFixups = map[string]string{
	"Judgment": "Judgement",
}

// loadNotes parses the card guide. path may be empty to use the embedded copy.
func loadNotes(path string) (*Guide, error) {
	var raw []byte
	var err error
	if path == "" {
		raw, err = assets.ReadFile("assets/tarot-notes.md")
	} else {
		raw, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, err
	}

	g := &Guide{ByName: make(map[string]*CardNotes, deckSize)}
	var cur *CardNotes
	var field *string // where plain paragraph text is being accumulated
	var inFence, inSketch bool
	var sketch []string

	// section adds a group to the outline; cards join the most recent one.
	section := func(title string) {
		g.Sections = append(g.Sections, Section{Title: title})
	}
	flush := func() {
		if cur != nil && cur.Name != "" {
			g.ByName[cur.Name] = cur
			if len(g.Sections) == 0 {
				section("Cards")
			}
			s := &g.Sections[len(g.Sections)-1]
			s.Cards = append(s.Cards, cur)
		}
		cur, field = nil, nil
	}

	// Everything before this marker is the how-to preamble, whose "### 1. ..."
	// headings would otherwise look like cards.
	const cardsMarker = "# The 78 Cards"
	started := false

	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()

		if !started {
			started = strings.HasPrefix(line, cardsMarker)
			continue
		}

		if strings.HasPrefix(line, "```") {
			// The first fenced block after a heading is the card's sketch.
			if inFence {
				inFence, inSketch = false, false
			} else {
				inFence = true
				inSketch = cur != nil && cur.Sketch == ""
				sketch = sketch[:0]
			}
			if !inFence && cur != nil && len(sketch) > 0 {
				cur.Sketch = strings.Join(sketch, "\n")
			}
			continue
		}
		if inFence {
			if inSketch {
				sketch = append(sketch, line)
			}
			continue
		}

		if name, label, ok := cardHeading(line); ok {
			flush()
			cur = &CardNotes{Name: name, Label: label}
			continue
		}
		// "## Major Arcana (Trumps)" and "### Suit of Wands" group the cards.
		if m := arcanaHeading.FindStringSubmatch(line); m != nil {
			flush()
			if !strings.Contains(m[1], "Minor") { // suits section themselves
				section(strings.TrimSpace(m[1]))
			}
			continue
		}
		if m := suitHeading.FindStringSubmatch(line); m != nil {
			flush()
			section(strings.TrimSpace(m[1]))
			continue
		}
		if cur == nil {
			continue
		}

		if m := waiteLine.FindStringSubmatch(line); m != nil {
			if m[1] == "Upright" {
				cur.WaiteUpright = m[2]
				field = &cur.WaiteUpright
			} else {
				cur.WaiteReversed = m[2]
				field = &cur.WaiteReversed
			}
			continue
		}
		if rest, ok := strings.CutPrefix(line, "> "); ok {
			appendText(field, rest)
			continue
		}
		if line == ">" {
			continue
		}

		switch {
		case strings.HasPrefix(line, "**Imagery:**"):
			cur.Imagery = labelValue(line, "**Imagery:**")
			field = &cur.Imagery
		case strings.HasPrefix(line, "**Upright:**"):
			cur.Upright = labelValue(line, "**Upright:**")
			field = &cur.Upright
		case strings.HasPrefix(line, "**Reversed:**"):
			cur.Reversed = labelValue(line, "**Reversed:**")
			field = &cur.Reversed
		case strings.HasPrefix(line, "**Mindful Tarot Interpretation:**"):
			cur.MindfulTitle = labelValue(line, "**Mindful Tarot Interpretation:**")
			field = &cur.MindfulBody
		case strings.HasPrefix(line, "---"):
			field = nil
		case strings.TrimSpace(line) == "":
			// Paragraph break: keep writing to the same field (the mindful
			// essay runs to several paragraphs), but mark the gap.
			appendText(field, "")
		default:
			appendText(field, line)
		}
	}
	flush()
	if err := sc.Err(); err != nil {
		return nil, err
	}

	for name, n := range g.ByName {
		// Fields absorb the blank line that ends them, so trim every one.
		for _, f := range []*string{&n.Sketch, &n.Imagery, &n.Upright, &n.Reversed,
			&n.MindfulTitle, &n.MindfulBody, &n.WaiteUpright, &n.WaiteReversed} {
			*f = strings.TrimSpace(*f)
		}
		if fixed, ok := notesNameFixups[name]; ok {
			delete(g.ByName, name)
			n.Label = strings.Replace(n.Label, name, fixed, 1)
			n.Name = fixed
			g.ByName[fixed] = n
		}
	}
	if len(g.ByName) != deckSize {
		return nil, fmt.Errorf("parsed %d cards from the guide, want %d", len(g.ByName), deckSize)
	}
	return g, nil
}

// cardHeading reports the card name and the heading as written, if the line is
// a card heading. Suit dividers and the how-to sections are not cards.
func cardHeading(line string) (name, label string, ok bool) {
	if m := majorHeading.FindStringSubmatch(line); m != nil {
		return strings.TrimSpace(m[2]), strings.TrimSpace(m[1]), true
	}
	if m := minorHeading.FindStringSubmatch(line); m != nil {
		return strings.TrimSpace(m[1]), strings.TrimSpace(m[1]), true
	}
	return "", "", false
}

func labelValue(line, label string) string {
	return strings.TrimSpace(strings.TrimPrefix(line, label))
}

// appendText adds a line to the field being accumulated, keeping blank lines as
// paragraph breaks and collapsing runs of them.
func appendText(field *string, line string) {
	if field == nil {
		return
	}
	line = strings.TrimSpace(line)
	if line == "" {
		if *field != "" && !strings.HasSuffix(*field, "\n\n") {
			*field += "\n\n"
		}
		return
	}
	if *field == "" || strings.HasSuffix(*field, "\n\n") {
		*field += line
		return
	}
	*field += " " + line
}
