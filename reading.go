package main

import (
	"fmt"
	"strconv"
	"strings"
)

// Position is one card in a reading, in the slot it landed in.
type Position struct {
	Label   string // where it sits: "Card 1: The Past", "Suit of Wands"
	Draw    int    // which draw it was from the pile, 0 outside freeform
	Meaning string // what the position itself signifies
	Card    Card
	Notes   *CardNotes
}

// heading names the card: the canonical name, with the trump's roman numeral,
// the reversal, and the draw appended when they apply.
func (p Position) heading() string {
	h := p.Card.Name
	if n := trumpNumeral(p.Notes, p.Card.Name); n != "" {
		h += " - " + n
	}
	if p.Card.Reversed {
		h += " in shadow (reversed)"
	}
	if p.Draw > 0 {
		h = fmt.Sprintf("%s (Draw %d)", h, p.Draw)
	}
	return h
}

// trumpNumeral is the roman numeral of a Major Arcana card, taken from the
// guide's own numbering ("12. The Hanged Man" gives XII). The suits are not
// numbered, and neither is the Fool, who is traditionally zero.
func trumpNumeral(notes *CardNotes, name string) string {
	if notes == nil || !strings.HasSuffix(notes.Label, name) {
		return ""
	}
	digits, _, ok := strings.Cut(notes.Label, ".")
	if !ok {
		return ""
	}
	n, err := strconv.Atoi(strings.TrimSpace(digits))
	if err != nil || n <= 0 {
		return ""
	}
	return roman(n)
}

// roman spells a number the way a tarot trump is numbered.
func roman(n int) string {
	var sb strings.Builder
	for _, step := range []struct {
		value int
		sign  string
	}{
		{1000, "M"}, {900, "CM"}, {500, "D"}, {400, "CD"},
		{100, "C"}, {90, "XC"}, {50, "L"}, {40, "XL"},
		{10, "X"}, {9, "IX"}, {5, "V"}, {4, "IV"}, {1, "I"},
	} {
		for n >= step.value {
			sb.WriteString(step.sign)
			n -= step.value
		}
	}
	return sb.String()
}

// Reading is a set of drawn cards.
type Reading struct {
	Query     string
	Spread    *Spread // nil in freeform and explore modes
	Positions []Position
}

// place puts a card in a position. def may be nil, for a card drawn outside of
// a spread.
func place(deck *Deck, c Card, def *PositionDef) Position {
	p := Position{Card: c, Notes: deck.notes(c)}
	if def != nil {
		p.Label = def.Name
		p.Meaning = def.Meaning
	}
	return p
}

// newReading deals a spread.
func newReading(deck *Deck, pile *Pile, s *Spread, query string, progress func(int)) *Reading {
	r := &Reading{Query: query, Spread: s, Positions: make([]Position, 0, s.Len())}
	for i := range s.Positions {
		if progress != nil {
			progress(i + 1)
		}
		card, ok := pile.Draw()
		if !ok {
			break
		}
		r.Positions = append(r.Positions, place(deck, card, &s.Positions[i]))
	}
	return r
}

// title is how the card is announced under its picture.
func (p Position) title() string { return p.Card.Title() }

// Title names the card and says whether it landed reversed.
func (c Card) Title() string {
	if c.Reversed {
		return c.Name + " in shadow (reversed)"
	}
	return c.Name
}
