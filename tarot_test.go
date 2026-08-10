package main

import (
	"bufio"
	"fmt"
	"io"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func testDeck(t *testing.T) *Deck {
	t.Helper()
	deck, err := loadDeck("")
	if err != nil {
		t.Fatal(err)
	}
	return deck
}

func TestLoadDeck(t *testing.T) {
	deck := testDeck(t)
	if got := len(deck.cards); got != deckSize*2 {
		t.Errorf("card table has %d entries, want %d", got, deckSize*2)
	}
	// Every card must be renderable and carry guide notes in both orientations.
	for n := 1; n <= deckSize*2; n++ {
		c := deck.card(n)
		if c.Name == "" {
			t.Fatalf("card %d missing from the table", n)
		}
		if (n > deckSize) != c.Reversed {
			t.Errorf("card %d reversed=%v, want %v", n, c.Reversed, n > deckSize)
		}
		if _, err := renderCard(c, 4, false); err != nil {
			t.Errorf("card %d (%s): %v", n, c.Name, err)
		}
	}
}

func TestNotesCoverEveryCard(t *testing.T) {
	deck := testDeck(t)
	if got := len(deck.guide.ByName); got != deckSize {
		t.Fatalf("guide has %d cards, want %d", got, deckSize)
	}
	for n := 1; n <= deckSize; n++ {
		c := deck.card(n)
		notes := deck.notes(c)
		if notes == nil {
			t.Fatalf("no guide entry for %s", c.Name)
		}
		for _, f := range []struct{ label, value string }{
			{"sketch", notes.Sketch},
			{"imagery", notes.Imagery},
			{"upright", notes.Upright},
			{"reversed", notes.Reversed},
			{"mindful title", notes.MindfulTitle},
			{"mindful body", notes.MindfulBody},
			{"waite upright", notes.WaiteUpright},
			{"waite reversed", notes.WaiteReversed},
		} {
			if strings.TrimSpace(f.value) == "" {
				t.Errorf("%s: empty %s", c.Name, f.label)
			}
		}
		// The essay is prose, not a stray heading or list fragment.
		if strings.Contains(notes.MindfulBody, "**Waite") || strings.HasPrefix(notes.MindfulBody, "#") {
			t.Errorf("%s: mindful body picked up neighboring markup", c.Name)
		}
	}
}

func TestNotesSpotCheck(t *testing.T) {
	deck := testDeck(t)
	fool := deck.guide.ByName["The Fool"]
	if fool == nil {
		t.Fatal("The Fool is missing from the guide")
	}
	if fool.MindfulTitle != "Beginner's Mind" {
		t.Errorf("mindful title = %q, want %q", fool.MindfulTitle, "Beginner's Mind")
	}
	if !strings.HasPrefix(fool.Upright, "It is a time of new beginnings.") {
		t.Errorf("upright summary = %q", truncate(fool.Upright))
	}
	if !strings.HasPrefix(fool.WaiteUpright, "Folly, mania, extravagance") {
		t.Errorf("waite upright = %q", truncate(fool.WaiteUpright))
	}
	if !strings.Contains(fool.Sketch, "'-------------'") {
		t.Errorf("sketch does not look like the card drawing: %q", truncate(fool.Sketch))
	}
	// "Judgment" in the guide is "Judgement" in the deck data.
	if deck.guide.ByName["Judgement"] == nil {
		t.Error("Judgement is missing from the guide")
	}
}

func truncate(s string) string {
	if len(s) > 60 {
		return s[:60] + "..."
	}
	return s
}

func TestPileDealsWithoutRepeating(t *testing.T) {
	deck := testDeck(t)
	for seed := int64(0); seed < 20; seed++ {
		pile := newPile(deck, rand.New(rand.NewSource(seed)), true)
		seen := map[string]bool{}
		for i := 0; i < deckSize; i++ {
			c, ok := pile.Draw()
			if !ok {
				t.Fatalf("seed %d: pile ran dry after %d cards", seed, i)
			}
			if seen[c.Name] {
				t.Fatalf("seed %d: %s dealt twice", seed, c.Name)
			}
			seen[c.Name] = true
		}
		// The next draw reshuffles rather than failing.
		if _, ok := pile.Draw(); !ok || pile.Shuffles != 2 {
			t.Errorf("seed %d: pile did not reshuffle (shuffles=%d)", seed, pile.Shuffles)
		}
	}
}

func TestNoReversals(t *testing.T) {
	deck := testDeck(t)
	pile := newPile(deck, rand.New(rand.NewSource(5)), false)
	for i := 0; i < deckSize; i++ {
		if c, _ := pile.Draw(); c.Reversed {
			t.Fatalf("%s came up reversed with reversals off", c.Name)
		}
	}
}

func TestSpreadsAreWellFormed(t *testing.T) {
	deck := testDeck(t)
	for key, s := range spreads {
		if s.Key != key {
			t.Errorf("spread %q has key %q", key, s.Key)
		}
		for i, def := range s.Positions {
			if def.Name == "" || def.Meaning == "" {
				t.Errorf("%s position %d is missing a name or meaning", key, i+1)
			}
		}

		rng := rand.New(rand.NewSource(11))
		r := newReading(deck, newPile(deck, rng, true), s, "q", nil)
		if len(r.Positions) != len(s.Positions) {
			t.Errorf("%s dealt %d cards, want %d", key, len(r.Positions), len(s.Positions))
		}
		text := plain(r, options{art: artNone})
		if strings.Count(text, divider) != len(s.Positions)+2 {
			t.Errorf("%s reading has %d dividers, want %d", key, strings.Count(text, divider), len(s.Positions)+2)
		}
	}
}

func TestPageIncludesGuideSummary(t *testing.T) {
	deck := testDeck(t)
	upright := place(deck, deck.card(1), &celticSpread.Positions[0]) // The Fool
	reversed := place(deck, deck.card(1+deckSize), nil)              // The Fool, reversed
	opts := options{art: artNone}

	up := page(upright, opts)
	if !strings.Contains(up, "Upright:") || strings.Contains(up, "Reversed:") {
		t.Error("upright page should carry the upright summary only")
	}
	if !strings.Contains(strings.ReplaceAll(up, "\n ", " "), "time of new beginnings") {
		t.Error("upright page is missing the guide summary")
	}
	down := page(reversed, opts)
	if !strings.Contains(down, "Reversed:") {
		t.Error("reversed page is missing the reversed summary")
	}
	// The essay stays behind the m key.
	if strings.Contains(up, "Ise Jingu") {
		t.Error("the mindful essay should not be printed with the card")
	}
	if !strings.Contains(mindful(upright), "Ise Jingu") {
		t.Error("the mindful reading is missing its essay")
	}
	if !strings.Contains(detail(upright), "Waite (1911):") {
		t.Error("detail is missing Waite")
	}
}

func TestSketchArtFlipsWhenReversed(t *testing.T) {
	deck := testDeck(t)
	up := sketch(place(deck, deck.card(1), nil))
	down := sketch(place(deck, deck.card(1+deckSize), nil))
	if up == "" || down == "" {
		t.Fatal("sketch art came out empty")
	}
	if up == down {
		t.Error("a reversed card should be drawn flipped")
	}
	upLines := strings.Split(strings.TrimRight(up, "\n"), "\n")
	downLines := strings.Split(strings.TrimRight(down, "\n"), "\n")
	if len(upLines) != len(downLines) || upLines[0] != downLines[len(downLines)-1] {
		t.Error("flipping should reverse the line order and nothing else")
	}
}

func TestReadingIsReproducible(t *testing.T) {
	deck := testDeck(t)
	read := func() string {
		rng := rand.New(rand.NewSource(99))
		return plain(newReading(deck, newPile(deck, rng, true), celticSpread, "q", nil), options{art: artNone})
	}
	first, second := read(), read()
	if first != second {
		t.Error("same seed produced different readings")
	}
}

func TestWrap(t *testing.T) {
	for _, tc := range []struct {
		in, want string
		width    int
	}{
		{"one two three", "one two\nthree", 8},
		{"exactlyten", "exactlyten", 5}, // a word longer than width is never split
		{"", "", 10},
		{"a  b", "a b", 10},
	} {
		if got := wrap(tc.in, tc.width); got != tc.want {
			t.Errorf("wrap(%q, %d) = %q, want %q", tc.in, tc.width, got, tc.want)
		}
	}

	long := wrap(celticSpread.Positions[0].Meaning, wrapWidth)
	for _, line := range strings.Split(long, "\n") {
		if len(line) > wrapWidth {
			t.Errorf("line exceeds %d chars: %q", wrapWidth, line)
		}
	}
}

func TestSeedFrom(t *testing.T) {
	if got := seedFrom(42, "anything"); got != 42 {
		t.Errorf("explicit seed = %d, want 42", got)
	}
	if seedFrom(0, "a") == seedFrom(0, "b") {
		t.Error("different queries produced the same seed")
	}
}

func TestGuideOutlineMatchesTheDeck(t *testing.T) {
	deck := testDeck(t)
	want := []string{
		"Major Arcana (Trumps)",
		"Suit of Wands", "Suit of Cups", "Suit of Swords", "Suit of Pentacles",
	}
	var got []string
	total := 0
	for _, s := range deck.guide.Sections {
		got = append(got, s.Title)
		total += len(s.Cards)
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("sections = %v, want %v", got, want)
	}
	if total != deckSize {
		t.Errorf("sections hold %d cards, want %d", total, deckSize)
	}
	if n := len(deck.guide.Sections[0].Cards); n != 22 {
		t.Errorf("Major Arcana has %d cards, want 22", n)
	}
	if first := deck.guide.Sections[0].Cards[0]; first.Label != "0. The Fool" {
		t.Errorf("first label = %q, want %q", first.Label, "0. The Fool")
	}
	// Every card in the outline resolves back to a card in the deck.
	for _, s := range deck.guide.Sections {
		for _, c := range s.Cards {
			if _, ok := deck.cardNamed(c.Name); !ok {
				t.Errorf("outline card %q is not in the deck", c.Name)
			}
		}
	}
}

func TestOutlineFoldingAndCursor(t *testing.T) {
	deck := testDeck(t)
	o := newOutline(deck.guide)
	// One heading per guide section, plus the synthetic Minor Arcana heading
	// that groups the four suits.
	wantRows := deckSize + len(deck.guide.Sections) + 1
	if len(o.rows) != wantRows {
		t.Fatalf("outline has %d rows, want %d", len(o.rows), wantRows)
	}
	if !o.rows[0].heading {
		t.Error("the outline should open with a section heading")
	}

	o.move(1)
	if o.rows[o.cursor].heading {
		t.Error("moving down from the heading should land on a card")
	}
	// Folding the Major Arcana hides its cards but keeps every heading.
	o.collapsed[0] = true
	vis := o.visible()
	if len(vis) != len(o.rows)-22 {
		t.Errorf("folded outline shows %d rows, want %d", len(vis), len(o.rows)-22)
	}

	// The cursor never walks off either end.
	o.collapsed[0] = false
	o.move(-50)
	if o.cursor != 0 {
		t.Errorf("cursor = %d after moving up past the top, want 0", o.cursor)
	}
	o.move(1000)
	if o.cursor != len(o.rows)-1 {
		t.Errorf("cursor = %d after moving past the end, want %d", o.cursor, len(o.rows)-1)
	}
}

func TestMinorArcanaGroupsTheSuits(t *testing.T) {
	deck := testDeck(t)
	o := newOutline(deck.guide)

	minorGroup := -1
	suitHeadings := 0
	for _, r := range o.rows {
		if r.heading && r.label == "Minor Arcana (Pips & Courts)" {
			minorGroup = r.section
		}
		if r.heading && strings.HasPrefix(r.label, "Suit of ") {
			suitHeadings++
			if r.parent != r.section && r.parent < 0 {
				t.Errorf("suit heading %q has no parent group", r.label)
			}
		}
	}
	if minorGroup < 0 {
		t.Fatal("no synthetic Minor Arcana heading in the outline")
	}
	if suitHeadings != 4 {
		t.Errorf("found %d suit headings, want 4", suitHeadings)
	}

	// Folding Minor Arcana hides every suit heading and every suit card —
	// only the Major Arcana's own heading and 22 cards remain, alongside the
	// Minor Arcana heading itself.
	o.collapsed[minorGroup] = true
	vis := o.visible()
	if want := 1 + 22 + 1; len(vis) != want {
		t.Errorf("outline with Minor Arcana folded shows %d rows, want %d", len(vis), want)
	}

	// Jumping to a card inside a folded suit unfolds both the suit and the
	// Minor Arcana heading over it.
	i := o.indexOf("Ace of Wands")
	if i < 0 {
		t.Fatal("Ace of Wands not found in the outline")
	}
	o.jump(i)
	if o.collapsed[minorGroup] {
		t.Error("jumping into a suit should unfold the Minor Arcana heading over it")
	}
	if o.cursor != i {
		t.Errorf("cursor = %d after jump, want %d", o.cursor, i)
	}
}

func TestPageIsCompact(t *testing.T) {
	deck := testDeck(t)
	p := place(deck, deck.card(1), nil)
	opts := options{art: artBoth, height: 24, color: false, cols: 80}
	lines := pageLines(p, opts)

	// The words ride in the right hand column beside the picture, so the page
	// is far shorter than the three blocks stacked would be.
	stacked := len(photoLines(p, opts)) + len(sketchLines(p, opts)) + len(cardText(p, opts, wrapWidth))
	if len(lines) >= stacked {
		t.Errorf("page is %d lines tall, no better than stacking them at %d", len(lines), stacked)
	}
	if len(lines) > len(photoLines(p, opts))+8 {
		t.Errorf("page is %d lines tall, want it to stay close to the picture's %d:\n%s",
			len(lines), len(photoLines(p, opts)), strings.Join(lines, "\n"))
	}
	// The name heads the page; the drawing and the words share the column
	// beside the picture, drawing first.
	joined := strings.Join(lines, "\n")
	name := strings.Index(joined, "The Fool")
	drawing := strings.Index(joined, "'-------------'")
	imagery := strings.Index(joined, "A young wanderer")
	if drawing < 0 || imagery < 0 || name < 0 {
		t.Fatalf("page is missing the drawing, the imagery, or the name:\n%s", joined)
	}
	if !(name < drawing && drawing < imagery) {
		t.Error("want the name, then the drawing, then the imagery")
	}
	// Nothing overruns the terminal.
	for _, l := range lines {
		if visibleWidth(l) > 80 {
			t.Errorf("line is %d columns wide: %q", visibleWidth(l), l)
		}
	}
}

func TestBareHidesTheWordsAndTheName(t *testing.T) {
	deck := testDeck(t)
	p := place(deck, deck.card(1), nil)
	p.Draw = 3
	bare := strings.Join(pageLines(p, options{art: artBoth, height: 8, cols: 80, bare: true}), "\n")
	for _, gone := range []string{"A young wanderer", "The Fool", "Draw 3", "'-------------'", "Upright:"} {
		if strings.Contains(bare, gone) {
			t.Errorf("picture-only page still shows %q", gone)
		}
	}
	if !strings.Contains(bare, divider) || len(strings.Split(bare, "\n")) < 8 {
		t.Error("picture-only page should still show the picture")
	}
}

func TestCardFlagPrintsPictureOnlyAndExits(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w

	runErr := run("", options{art: artPhoto, color: false}, runFlags{card: true, seed: 1})
	w.Close()
	os.Stdout = old
	if runErr != nil {
		t.Fatalf("run: %v", runErr)
	}

	out, _ := io.ReadAll(r)
	got := string(out)
	for _, gone := range []string{"Drawing card number", divider, "Upright:", "Reversed:", "Waite (1911)"} {
		if strings.Contains(got, gone) {
			t.Errorf("-card output still shows %q", gone)
		}
	}
	if lines := strings.Split(strings.TrimRight(got, "\n"), "\n"); len(lines) != cardArtHeight {
		t.Errorf("-card printed %d lines, want %d", len(lines), cardArtHeight)
	}
}

func TestHeadingNamesTheCardOnce(t *testing.T) {
	deck := testDeck(t)
	// The Fool is zero, which has no numeral.
	upright := place(deck, deck.card(1), &celticSpread.Positions[0])
	if got := upright.heading(); got != "The Fool" {
		t.Errorf("heading = %q", got)
	}
	reversed := place(deck, deck.card(1+deckSize), nil)
	reversed.Draw = 17
	if got := reversed.heading(); got != "The Fool in shadow (reversed) (Draw 17)" {
		t.Errorf("reversed draw heading = %q", got)
	}
	// The trumps carry a roman numeral; the suits carry nothing.
	for name, want := range map[string]string{
		"The Hanged Man": "The Hanged Man - XII",
		"Judgement":      "Judgement - XX",
		"The World":      "The World - XXI",
		"The Magician":   "The Magician - I",
		"Ace of Wands":   "Ace of Wands",
		"King of Cups":   "King of Cups",
	} {
		c, ok := deck.cardNamed(name)
		if !ok {
			t.Fatalf("%s is not in the deck", name)
		}
		if got := place(deck, c, nil).heading(); got != want {
			t.Errorf("heading = %q, want %q", got, want)
		}
	}

	// The name is in the heading and nowhere else on the page.
	text := page(upright, options{art: artNone, cols: 80})
	if n := strings.Count(text, "The Fool"); n != 1 {
		t.Errorf("the name appears %d times on the page, want once:\n%s", n, text)
	}
	if !strings.HasPrefix(text, "  The Fool\n  Card 1: The Present or The Self\n") {
		t.Errorf("want the name then its position at the top, got:\n%s", text)
	}
}

func TestVisibleWidthIgnoresColor(t *testing.T) {
	if got := visibleWidth("\x1b[38;2;1;2;3mab\x1b[0m"); got != 2 {
		t.Errorf("visibleWidth = %d, want 2", got)
	}
	rows := sideBySide([]string{"\x1b[31mxx\x1b[0m"}, []string{"y"}, 2)
	if want := "\x1b[31mxx\x1b[0m  y"; rows[0] != want {
		t.Errorf("sideBySide = %q, want %q", rows[0], want)
	}
}

func TestPageLeadsWithImagery(t *testing.T) {
	deck := testDeck(t)
	p := place(deck, deck.card(1), nil)
	text := page(p, options{art: artNone, cols: 80})
	imagery := strings.Index(text, "A young wanderer")
	upright := strings.Index(text, "Upright:")
	if imagery < 0 || upright < 0 {
		t.Fatalf("page is missing the imagery or the summary:\n%s", text)
	}
	if imagery > upright {
		t.Error("the imagery should lead the words, before the summary")
	}
	// The bash version's generated sentence is gone.
	if strings.Contains(text, "# The Fool in light") {
		t.Error("the generated interpretation should no longer be printed")
	}
}

func TestKeyDecoding(t *testing.T) {
	tr := &term{in: bufio.NewReader(strings.NewReader("\x1b[A\x1b[B\x1b[C\x1b[D\x1b[5~j\r\x1b")), raw: true}
	want := []string{keyUp, keyDown, keyRight, keyLeft, keyPgUp, "j", keyEnter, keyEsc}
	for _, w := range want {
		got, ok := tr.key()
		if !ok {
			t.Fatalf("input ran out before %q", w)
		}
		if got != w {
			t.Errorf("key = %q, want %q", got, w)
		}
	}
	if _, ok := tr.key(); ok {
		t.Error("expected end of input")
	}
}

func TestLineModeKeys(t *testing.T) {
	tr := &term{in: bufio.NewReader(strings.NewReader("\nj\nDOWN\nq\n"))}
	want := []string{keyEnter, "j", "down", keyQuit}
	for _, w := range want {
		if got, ok := tr.key(); !ok || got != w {
			t.Errorf("key = %q (ok=%v), want %q", got, ok, w)
		}
	}
}

func TestExploreCardStepping(t *testing.T) {
	deck := testDeck(t)
	o := newOutline(deck.guide)
	o.moveCard(1)
	if got := o.rows[o.cursor].label; got != "0. The Fool" {
		t.Fatalf("first card = %q, want %q", got, "0. The Fool")
	}
	// Left at the very first card stays put rather than landing on a heading.
	o.moveCard(-1)
	if got := o.rows[o.cursor].label; got != "0. The Fool" {
		t.Errorf("stepping back from the first card went to %q", got)
	}
	// Stepping forward crosses the section boundary onto the next card.
	for range 21 {
		o.moveCard(1)
	}
	if got := o.rows[o.cursor].label; got != "21. The World" {
		t.Fatalf("last major = %q, want %q", got, "21. The World")
	}
	o.moveCard(1)
	if got := o.rows[o.cursor]; got.heading || got.label != "Ace of Wands" {
		t.Errorf("card after the majors = %q (heading=%v), want Ace of Wands", got.label, got.heading)
	}
	o.moveCard(-1)
	if got := o.rows[o.cursor].label; got != "21. The World" {
		t.Errorf("stepping back across the boundary went to %q", got)
	}
	// And it never lands on a heading anywhere in the deck.
	o.cursor = 0
	o.moveCard(1)
	for range deckSize * 2 {
		o.moveCard(1)
		if o.rows[o.cursor].heading {
			t.Fatalf("landed on the heading %q", o.rows[o.cursor].label)
		}
	}
}

func TestBackspaceIsItsOwnKey(t *testing.T) {
	tr := &term{in: bufio.NewReader(strings.NewReader("\x7f\x08\x1b[D")), raw: true}
	want := []string{keyBksp, keyBksp, keyLeft}
	for _, w := range want {
		if got, ok := tr.key(); !ok || got != w {
			t.Errorf("key = %q (ok=%v), want %q", got, ok, w)
		}
	}
}

func TestPickModeResolvesTheCarousel(t *testing.T) {
	for _, tc := range []struct {
		flag    string
		dwell   time.Duration
		want    mode
		ordered bool
		wantDur time.Duration
	}{
		{"celtic", 0, modeCeltic, true, 0},
		{"explore", 0, modeExplore, true, 0},
		{"freeform", 0, modeFreeform, true, 0},
		// A dwell turns the browsing modes into the carousel.
		{"explore", 30 * time.Second, modeCarousel, true, 30 * time.Second},
		{"freeform", 30 * time.Second, modeCarousel, false, 30 * time.Second},
		// The carousel names itself, and picks up the default pace.
		{"carousel", 0, modeCarousel, true, defaultDwell},
		{"CAROUSEL", 5 * time.Second, modeCarousel, true, 5 * time.Second},
	} {
		u := &ui{opts: options{dwell: tc.dwell}}
		got, err := pickMode(u, tc.flag, runFlags{})
		if err != nil {
			t.Fatalf("-mode %s -dwell %s: %v", tc.flag, tc.dwell, err)
		}
		if got.mode != tc.want || got.dwell != tc.wantDur {
			t.Errorf("-mode %s -dwell %s = %v/%s, want %v/%s", tc.flag, tc.dwell, got.mode, got.dwell, tc.want, tc.wantDur)
		}
		if got.mode == modeCarousel && got.ordered != tc.ordered {
			t.Errorf("-mode %s: ordered = %v, want %v", tc.flag, got.ordered, tc.ordered)
		}
	}
	if _, err := pickMode(&ui{}, "tea", runFlags{}); err == nil {
		t.Error("an unknown mode should be an error")
	}
}

func TestCarouselStatus(t *testing.T) {
	got := carouselStatus(3, 9, 45*time.Second, false, false, true, 0, 0)
	if !strings.HasPrefix(got, " 3/9 · 45s · ") {
		t.Errorf("status = %q", got)
	}
	if paused := carouselStatus(1, 1, time.Minute, true, false, true, 0, 0); !strings.Contains(paused, "paused") {
		t.Errorf("paused status = %q", paused)
	}
	if rev := carouselStatus(1, 1, time.Minute, false, true, true, 0, 0); !strings.Contains(rev, "reversed") {
		t.Errorf("reversed status = %q", rev)
	}
}

func TestKeyWithinTimesOut(t *testing.T) {
	// Nothing to read: the wait ends on the clock, and input is still open.
	tr := &term{in: bufio.NewReader(&blockingReader{})}
	start := time.Now()
	if key, ok, timedOut := tr.keyWithin(40 * time.Millisecond); !timedOut || !ok || key != "" {
		t.Errorf("keyWithin = %q/%v/%v, want a timeout", key, ok, timedOut)
	}
	if elapsed := time.Since(start); elapsed < 30*time.Millisecond {
		t.Errorf("returned after %s, want to have waited", elapsed)
	}
	// A key that is already there comes back without waiting.
	tr2 := &term{in: bufio.NewReader(strings.NewReader("j\n"))}
	if key, ok, timedOut := tr2.keyWithin(time.Minute); key != "j" || !ok || timedOut {
		t.Errorf("keyWithin = %q/%v/%v, want j", key, ok, timedOut)
	}
}

// blockingReader never returns, standing in for an idle keyboard.
type blockingReader struct{}

func (b *blockingReader) Read(p []byte) (int, error) { select {} }

// menuKey is the number a mode sits at in the picker.
func menuKey(m mode) string {
	for i, e := range modeMenu {
		if e.mode == m {
			return strconv.Itoa(i + 1)
		}
	}
	return "0"
}

func TestReversalsChoiceReachesThePile(t *testing.T) {
	deck := testDeck(t)
	// The picker path: free form, a blank query, then "n" to the reversed
	// cards question.
	u := &ui{
		t:    &term{in: bufio.NewReader(strings.NewReader(menuKey(modeFreeform) + "\n\nn\n"))},
		deck: deck,
		pile: newPile(deck, rand.New(rand.NewSource(1)), true),
	}
	p, ok := chooseMode(u)
	if !ok || p.mode != modeFreeform {
		t.Fatalf("chooseMode = %v/%v, want freeform", p.mode, ok)
	}
	if p.reversals == nil || *p.reversals {
		t.Fatalf("reversals = %v, want it turned off", p.reversals)
	}

	// A carousel drawn at random asks too; in order does not.
	u.t = &term{in: bufio.NewReader(strings.NewReader(menuKey(modeCarousel) + "\n2\ny\n30\n"))}
	u.pile = newPile(deck, rand.New(rand.NewSource(1)), false)
	p, ok = chooseMode(u)
	if !ok || p.mode != modeCarousel || p.ordered {
		t.Fatalf("chooseMode = %v/%v ordered=%v, want a random carousel", p.mode, ok, p.ordered)
	}
	if p.reversals == nil || !*p.reversals {
		t.Fatalf("reversals = %v, want it turned on", p.reversals)
	}
	if p.dwell != 30*time.Second {
		t.Errorf("dwell = %s, want 30s", p.dwell)
	}

	u.t = &term{in: bufio.NewReader(strings.NewReader(menuKey(modeCarousel) + "\n1\n\n"))}
	p, _ = chooseMode(u)
	if !p.ordered || p.reversals != nil {
		t.Errorf("an ordered carousel should not ask about reversals (got %v)", p.reversals)
	}

	// q leaves without a mode.
	u.t = &term{in: bufio.NewReader(strings.NewReader("q\n"))}
	if p, ok := chooseMode(u); ok || p.mode != "" {
		t.Errorf("q at the menu = %v/%v, want a quit", p.mode, ok)
	}
}

func TestOutlineSearch(t *testing.T) {
	deck := testDeck(t)
	o := newOutline(deck.guide)

	i := o.find("tower", 0, 1)
	if i < 0 || o.rows[i].label != "16. The Tower" {
		t.Fatalf("find(tower) = %d (%q)", i, o.rows[i].label)
	}
	// Case does not matter, and headings are never a match.
	if j := o.find("THE TOWER", 0, 1); j != i {
		t.Errorf("find is case sensitive: %d vs %d", j, i)
	}
	if h := o.find("suit of wands", 0, 1); h >= 0 {
		t.Errorf("find matched the heading %q", o.rows[h].label)
	}
	if none := o.find("hierophantom", 0, 1); none != -1 {
		t.Errorf("find(nonsense) = %d, want -1", none)
	}

	// Searching wraps around, forwards and backwards.
	first := o.find("of cups", 0, 1)
	last := o.find("of cups", 0, -1)
	if first < 0 || last < 0 || first == last {
		t.Fatalf("wrapping search: forward %d, backward %d", first, last)
	}
	if o.find("of cups", last, 1) != first {
		t.Error("searching forward from the last match should wrap to the first")
	}

	// Jumping to a match opens whatever section was folded over it.
	o.collapsed[0] = true
	o.jump(i)
	if o.collapsed[0] || o.cursor != i {
		t.Errorf("jump left the section folded (cursor=%d, folded=%v)", o.cursor, o.collapsed[0])
	}
}

func TestSearchOutlineIsIncremental(t *testing.T) {
	deck := testDeck(t)

	// Typing "tower" one letter at a time lands on it without an Enter; the
	// query comes back for n/N to repeat.
	u := &ui{t: &term{in: bufio.NewReader(strings.NewReader("tower\r")), raw: true, rows: 24, cols: 80}}
	o := newOutline(deck.guide)
	q, found, canceled, ok := searchOutline(u, o)
	if !ok || canceled || !found || q != "tower" {
		t.Fatalf("searchOutline = %q found=%v canceled=%v ok=%v", q, found, canceled, ok)
	}
	if o.rows[o.cursor].label != "16. The Tower" {
		t.Errorf("cursor landed on %q, want The Tower", o.rows[o.cursor].label)
	}

	// Esc cancels back to wherever the cursor started.
	start := 5
	o.cursor = start
	u.t = &term{in: bufio.NewReader(strings.NewReader("fool\x1b")), raw: true, rows: 24, cols: 80}
	if _, _, canceled, ok := searchOutline(u, o); !ok || !canceled {
		t.Errorf("esc should cancel, got canceled=%v ok=%v", canceled, ok)
	}
	if o.cursor != start {
		t.Errorf("cursor after cancel = %d, want back at %d", o.cursor, start)
	}

	// Backspace un-types a letter and the search moves back with it.
	o.cursor = 0
	u.t = &term{in: bufio.NewReader(strings.NewReader("fooler\x7f\x7f\r")), raw: true, rows: 24, cols: 80}
	if q, found, _, ok := searchOutline(u, o); !ok || !found || q != "fool" {
		t.Fatalf("searchOutline after backspacing = %q found=%v ok=%v", q, found, ok)
	}
	if got := o.rows[o.cursor].label; got != "0. The Fool" {
		t.Errorf("cursor landed on %q, want The Fool", got)
	}
}

func TestCardPageHasNoSectionSubtitle(t *testing.T) {
	deck := testDeck(t)
	p := place(deck, deck.card(1), nil) // as explore and the carousel show it
	text := page(p, options{art: artNone, cols: 80})
	for _, gone := range []string{"Major Arcana", "Trumps", "Suit of"} {
		if strings.Contains(text, gone) {
			t.Errorf("page still carries the %q subtitle:\n%s", gone, text)
		}
	}
	if !strings.HasPrefix(text, "  The Fool\n \n") {
		t.Errorf("want the heading alone at the top, got:\n%s", text)
	}
}

func TestEverySpreadHasALayout(t *testing.T) {
	for key, s := range spreads {
		if !hasLayout(s) {
			t.Errorf("spread %q has no tableau layout", key)
		}
		if len(layouts[key]) != len(s.Positions) {
			t.Errorf("spread %q has %d positions but %d slots", key, len(s.Positions), len(layouts[key]))
		}
		// No two positions may share a slot.
		seen := map[slot]int{}
		for i, sl := range layouts[key] {
			if prev, dup := seen[sl]; dup {
				t.Errorf("spread %q: positions %d and %d share slot %v", key, prev+1, i+1, sl)
			}
			seen[sl] = i
		}
	}
}

func TestTableauPlacesEveryCard(t *testing.T) {
	deck := testDeck(t)
	rng := rand.New(rand.NewSource(4))
	r := newReading(deck, newPile(deck, rng, true), celticSpread, "q", nil)
	opts := options{art: artPhoto, cols: 110, rows: 50}
	lines := strings.Split(tableau(r, opts), "\n")

	// Every card is captioned, once.
	for i := range r.Positions {
		want := fmt.Sprintf("%d.", i+1)
		n := 0
		for _, l := range lines {
			n += strings.Count(l, want)
		}
		if n != 1 {
			t.Errorf("caption %q appears %d times, want once", want, n)
		}
	}

	// The bands hold what the Celtic cross expects: the cross around the
	// centre, the staff climbing beside it.
	band := func(caption string) int {
		for i, l := range lines {
			if strings.Contains(l, caption) {
				return i
			}
		}
		return -1
	}
	centre := band("1.")
	if centre < 0 {
		t.Fatal("the middle of the cross is missing")
	}
	for _, alongside := range []string{"2.", "3.", "4.", "9."} {
		if band(alongside) != centre {
			t.Errorf("%q should sit on the centre band", alongside)
		}
	}
	if above, below := band("5."), band("6."); !(above < centre && centre < below) {
		t.Errorf("want 5 above and 6 below the centre, got %d/%d/%d", above, centre, below)
	}
	// The staff reads bottom to top: 10 at the head, 7 at the foot.
	if !(band("10.") < band("9.") && band("9.") < band("8.") && band("8.") < band("7.")) {
		t.Error("the staff should climb from 7 at the foot to 10 at the head")
	}
	// Nothing overruns the terminal.
	for _, l := range lines {
		if visibleWidth(l) > opts.cols {
			t.Errorf("tableau line is %d columns wide: %q", visibleWidth(l), l)
		}
	}
}

func TestThreeCardTableauIsOneRow(t *testing.T) {
	deck := testDeck(t)
	for _, s := range []*Spread{threeSpread} {
		rng := rand.New(rand.NewSource(7))
		r := newReading(deck, newPile(deck, rng, true), s, "", nil)
		lines := strings.Split(tableau(r, options{art: artPhoto, cols: 100, rows: 40}), "\n")
		row := -1
		for i, l := range lines {
			if strings.Contains(l, "1.") {
				row = i
			}
		}
		if row < 0 {
			t.Fatalf("%s: no cards in the tableau", s.Key)
		}
		if !strings.Contains(lines[row], "2.") || !strings.Contains(lines[row], "3.") {
			t.Errorf("%s: want all three cards on one band, got %q", s.Key, lines[row])
		}
	}
}

func TestSpreadTitlesAndKeys(t *testing.T) {
	want := []string{"celtic", "three"}
	var got []string
	for _, s := range spreadOrder {
		got = append(got, s.Key)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("spreads = %v, want %v", got, want)
	}
	// The menu offers each of them, then the browsing modes.
	if len(modeMenu) != len(spreadOrder)+3 {
		t.Errorf("menu has %d entries, want %d", len(modeMenu), len(spreadOrder)+3)
	}
	for i, s := range spreadOrder {
		if modeMenu[i].mode != mode(s.Key) {
			t.Errorf("menu slot %d = %q, want %q", i+1, modeMenu[i].mode, s.Key)
		}
	}
	for _, key := range want {
		u := &ui{opts: options{}}
		if p, err := pickMode(u, key, runFlags{}); err != nil || p.mode != mode(key) {
			t.Errorf("-mode %s = %v (%v)", key, p.mode, err)
		}
	}
}

func TestRoman(t *testing.T) {
	for n, want := range map[int]string{
		1: "I", 4: "IV", 5: "V", 9: "IX", 12: "XII", 14: "XIV",
		19: "XIX", 20: "XX", 21: "XXI",
	} {
		if got := roman(n); got != want {
			t.Errorf("roman(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestSliceVisibleKeepsColour(t *testing.T) {
	line := "\x1b[31ma\x1b[32mb\x1b[33mc\x1b[0m"
	got := sliceVisible(line, 1, 1)
	if visibleWidth(got) != 1 || !strings.Contains(got, "b") || strings.Contains(got, "a") {
		t.Errorf("sliceVisible = %q, want just b", got)
	}
	// The escapes survive, so the colour of the kept character is still set.
	if !strings.Contains(got, "\x1b[32m") {
		t.Errorf("sliceVisible dropped the colour: %q", got)
	}
	if got := sliceVisible("plain text", 6, 4); got != "text" {
		t.Errorf("sliceVisible = %q, want %q", got, "text")
	}
}

func TestTableauCardsAreTheSameSizeInEverySpread(t *testing.T) {
	deck := testDeck(t)
	opts := options{art: artPhoto, cols: 100, rows: 40}
	size := func(s *Spread) int {
		rng := rand.New(rand.NewSource(2))
		r := newReading(deck, newPile(deck, rng, true), s, "", nil)
		lines := strings.Split(tableau(r, opts), "\n")
		// The first band of art runs from the banner to its first caption.
		caption := regexp.MustCompile(`^\s*\d+\.`)
		art := 0
		for _, l := range lines[5:] {
			if caption.MatchString(l) {
				break
			}
			art++
		}
		return art
	}
	if celtic, three := size(celticSpread), size(threeSpread); celtic != three {
		t.Errorf("the Celtic cross draws %d rows of art, a three card reading %d: they should match",
			celtic, three)
	}
}

func TestCardKey(t *testing.T) {
	// With ten cards, 0 stands in for the tenth.
	for key, want := range map[string]int{"1": 0, "4": 3, "9": 8, "0": 9} {
		if got, ok := cardKey(key, 10); !ok || got != want {
			t.Errorf("cardKey(%q, 10) = %d/%v, want %d", key, got, ok, want)
		}
	}
	// A three card spread only answers to 1, 2 and 3.
	if _, ok := cardKey("4", 3); ok {
		t.Error("cardKey(4, 3) should not name a card")
	}
	if _, ok := cardKey("0", 3); ok {
		t.Error("cardKey(0, 3) should not name a card")
	}
	for _, key := range []string{"a", keyEnter, keyLeft, "", "10"} {
		if _, ok := cardKey(key, 10); ok {
			t.Errorf("cardKey(%q) should not name a card", key)
		}
	}
	if got := cardKeys(10); got != "1-9 and 0" {
		t.Errorf("cardKeys(10) = %q", got)
	}
	if got := cardKeys(3); got != "1-3" {
		t.Errorf("cardKeys(3) = %q", got)
	}
}

func TestEveryShuffledModeAsksAboutReversals(t *testing.T) {
	deck := testDeck(t)
	// Celtic cross, three card and free form all deal off a shuffled pile.
	for _, m := range []mode{modeCeltic, "three", modeFreeform} {
		u := &ui{
			t:    &term{in: bufio.NewReader(strings.NewReader(menuKey(m) + "\n\nn\n"))},
			deck: deck,
			pile: newPile(deck, rand.New(rand.NewSource(1)), true),
		}
		p, ok := chooseMode(u)
		if !ok || p.mode != m {
			t.Fatalf("chooseMode = %v/%v, want %v", p.mode, ok, m)
		}
		if p.reversals == nil || *p.reversals {
			t.Errorf("%s: reversals = %v, want the answer to have been taken", m, p.reversals)
		}
	}
	// Explore walks the deck in order, so it has nothing to ask.
	u := &ui{
		t:    &term{in: bufio.NewReader(strings.NewReader(menuKey(modeExplore) + "\n"))},
		deck: deck,
		pile: newPile(deck, rand.New(rand.NewSource(1)), true),
	}
	if p, ok := chooseMode(u); !ok || p.reversals != nil {
		t.Errorf("explore asked about reversals (%v)", p.reversals)
	}
}

func TestMenuArrowSelection(t *testing.T) {
	// Two downs and Enter picks the third entry.
	tr := &term{in: bufio.NewReader(strings.NewReader("\x1b[B\x1b[B\r")), raw: true, rows: 24, cols: 80}
	items := []menuItem{{"One", ""}, {"Two", ""}, {"Three", ""}}
	if i, ok := tr.menu("pick", items, ""); !ok || i != 2 {
		t.Errorf("menu = %d/%v, want the third entry", i, ok)
	}
	// A number still jumps straight there, and the cursor stops at the ends.
	tr = &term{in: bufio.NewReader(strings.NewReader("2")), raw: true, rows: 24, cols: 80}
	if i, ok := tr.menu("pick", items, ""); !ok || i != 1 {
		t.Errorf("menu = %d/%v, want the second entry", i, ok)
	}
	tr = &term{in: bufio.NewReader(strings.NewReader("kkkk\r")), raw: true, rows: 24, cols: 80}
	if i, ok := tr.menu("pick", items, ""); !ok || i != 0 {
		t.Errorf("menu = %d/%v, want to stop at the first entry", i, ok)
	}
	// q backs out.
	tr = &term{in: bufio.NewReader(strings.NewReader("q")), raw: true, rows: 24, cols: 80}
	if _, ok := tr.menu("pick", items, ""); ok {
		t.Error("q should back out of the menu")
	}
}

func TestFlagsReachEveryModeWithoutTheMenu(t *testing.T) {
	// -random makes the carousel draw from the pile; without it, in order.
	for _, tc := range []struct {
		flag    string
		f       runFlags
		want    mode
		ordered bool
	}{
		{"celtic", runFlags{}, modeCeltic, true},
		{"three", runFlags{}, "three", true},
		{"freeform", runFlags{}, modeFreeform, true},
		{"explore", runFlags{}, modeExplore, true},
		{"carousel", runFlags{}, modeCarousel, true},
		{"carousel", runFlags{random: true}, modeCarousel, false},
	} {
		u := &ui{opts: options{}}
		p, err := pickMode(u, tc.flag, tc.f)
		if err != nil {
			t.Fatalf("-mode %s: %v", tc.flag, err)
		}
		if p.mode != tc.want || p.ordered != tc.ordered {
			t.Errorf("-mode %s random=%v = %v/ordered=%v, want %v/%v",
				tc.flag, tc.f.random, p.mode, p.ordered, tc.want, tc.ordered)
		}
		// Nothing was asked, so the flags stand as given.
		if p.reversals != nil {
			t.Errorf("-mode %s should not have asked about reversals", tc.flag)
		}
	}
}
