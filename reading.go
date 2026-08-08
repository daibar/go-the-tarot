package main

// Position is one card in a reading, in the slot it landed in.
type Position struct {
	Name    string // "Card 1: The Present or The Self", empty outside a spread
	Meaning string // what the position itself signifies
	Card    Card
	Notes   *CardNotes
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
		p.Name = def.Name
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
