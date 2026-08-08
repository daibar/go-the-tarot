package main

import (
	"bufio"
	"bytes"
	"embed"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
)

// The deck is 78 cards. Card numbers 1-78 are upright ("light"), and the same
// card reversed ("shadow") is numbered card+78, exactly as in number_cards.dat.
const deckSize = 78

//go:embed assets/number_cards.dat assets/tarot-notes.md assets/img/*.jpg
var assets embed.FS

// Card is one entry from number_cards.dat.
type Card struct {
	Number   int    // 1-156
	Name     string // "The Fool"
	Aspect   string // "light" or "shadow"
	Reversed bool
}

// ImageNumber is the number of the jpg for this card; both the upright and the
// reversed entry share the upright image (the reversed one is flipped later).
func (c Card) ImageNumber() int {
	if c.Number > deckSize {
		return c.Number - deckSize
	}
	return c.Number
}

// Deck holds the card table and the card guide, keyed by card name.
type Deck struct {
	cards  map[int]Card
	byName map[string]int // card name to its upright number
	guide  *Guide
}

// loadDeck reads the card data. notesPath may be empty to use the embedded
// copy of the card guide.
func loadDeck(notesPath string) (*Deck, error) {
	d := &Deck{
		cards:  make(map[int]Card, deckSize*2),
		byName: make(map[string]int, deckSize),
	}

	raw, err := assets.ReadFile("assets/number_cards.dat")
	if err != nil {
		return nil, err
	}
	sc := bufio.NewScanner(bytes.NewReader(raw))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		parts := strings.Split(line, "=")
		if len(parts) != 3 {
			return nil, fmt.Errorf("malformed card line %q", line)
		}
		num, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, fmt.Errorf("malformed card number in %q: %w", line, err)
		}
		d.cards[num] = Card{
			Number:   num,
			Name:     parts[1],
			Aspect:   parts[2],
			Reversed: parts[2] == "shadow",
		}
		if num <= deckSize {
			d.byName[parts[1]] = num
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	if d.guide, err = loadNotes(notesPath); err != nil {
		return nil, fmt.Errorf("reading the card guide: %w", err)
	}

	// The card table and the guide are keyed by card name, so a spelling drift
	// between them would silently produce a card with no meaning. Catch it up
	// front rather than printing a blank reading.
	for n := 1; n <= deckSize*2; n++ {
		c, ok := d.cards[n]
		if !ok {
			return nil, fmt.Errorf("card %d missing from number_cards.dat", n)
		}
		if _, ok := d.guide.ByName[c.Name]; !ok {
			return nil, fmt.Errorf("no guide entry for %q (card %d)", c.Name, n)
		}
	}
	return d, nil
}

// notes returns the guide entry for a card.
func (d *Deck) notes(c Card) *CardNotes { return d.guide.ByName[c.Name] }

// cardNamed looks a card up by name, upright.
func (d *Deck) cardNamed(name string) (Card, bool) {
	n, ok := d.byName[name]
	if !ok {
		return Card{}, false
	}
	return d.cards[n], true
}

func (d *Deck) card(number int) Card { return d.cards[number] }

// reversed returns the same card turned the other way up.
func (d *Deck) reversed(c Card, reversed bool) Card {
	n := c.ImageNumber()
	if reversed {
		n += deckSize
	}
	return d.cards[n]
}

// Pile is a shuffled deck dealt without replacement. Freeform readings keep
// drawing from it, so it reshuffles once it runs out.
type Pile struct {
	deck      *Deck
	rng       *rand.Rand
	order     []int // upright card numbers, 1-78
	next      int
	reversals bool
	Shuffles  int
}

func newPile(deck *Deck, rng *rand.Rand, reversals bool) *Pile {
	p := &Pile{deck: deck, rng: rng, reversals: reversals}
	p.Shuffle()
	return p
}

func (p *Pile) Shuffle() {
	perm := p.rng.Perm(deckSize)
	p.order = make([]int, deckSize)
	for i, v := range perm {
		p.order[i] = v + 1
	}
	p.next = 0
	p.Shuffles++
}

// Remaining is how many cards are left before the pile must be reshuffled.
func (p *Pile) Remaining() int { return len(p.order) - p.next }

// Draw takes the next card off the pile, orienting it upright or reversed. The
// second return is false only if the pile is somehow empty.
func (p *Pile) Draw() (Card, bool) {
	if p.Remaining() == 0 {
		p.Shuffle()
	}
	if p.Remaining() == 0 {
		return Card{}, false
	}
	n := p.order[p.next]
	p.next++
	if p.reversals && p.rng.Intn(2) == 1 {
		n += deckSize
	}
	return p.deck.card(n), true
}
