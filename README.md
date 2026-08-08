go-the-tarot
============

A Go port of [bash-the-tarot](https://github.com/uriel1998/bash-the-tarot) by Steven Saus:
a command line tarot reader with ASCII card images, grown into a small terminal
reader for the card guide in `~/notes/tarot.md`.

A single static binary with no runtime dependencies — `jp2a`, `fzf`, and `jq`
are gone, the image rendering and the interface are built in, and the deck data,
the card guide, and all 78 card images are embedded with `go:embed`.

## Build

    go build -o tarot .

Requires Go 1.22+. No modules to download.

## Usage

    ./tarot [flags] [your question here]

Any words you pass are your query; they are folded into the random seed, so
asking the same question at a different moment still gives a different reading.
With no `-mode` flag, it asks how you want to read.

### Modes

| mode | what it does |
| --- | --- |
| `celtic` | the ten card Celtic cross, the layout the bash version drew |
| `three` | past, present, future |
| `freeform` | keep drawing from the pile for as long as you like; it reshuffles when the deck runs out |
| `explore` | browse the deck through the guide's own outline — no shuffle, no question |
| `carousel` | cards turn over on their own on a timer, in order or drawn at random |

Spreads deal the cards, walk you through them one at a time, then offer a review
menu where you can revisit any card by number, export with `x`, or quit with `q`.

### What is on a card

    Card 1: The Past

    <the deck scan as colored ASCII>   <the guide's line drawing>

    A young wanderer in an embroidered tunic steps toward a cliff edge...

      The Fool

    The Past - What has already happened and still shapes the question...

    Upright: It is a time of new beginnings. Step forward with an open heart...

The imagery sits just above the card's name, and the summary underneath is the
guide's, matched to whether the card landed upright or reversed.

### Keys

Text scrolls with vi keys everywhere — cards, essays, and the outline:

| key | what it does |
| --- | --- |
| `j` / `k`, `↓` / `↑` | line down / up |
| `d` / `u` | half page down / up |
| `space` / `b`, `PgDn` / `PgUp` | page down / up |
| `g` / `G`, `Home` / `End` | top / bottom |

While a card is up:

| key | what it shows |
| --- | --- |
| `Enter` | continue |
| `m` | the mindful reading — the full contemplative essay for that card |
| `w` | toggle Waite's 1911 divinatory meaning |
| `q` | quit |

In explore mode the arrows steer the outline: `↑`/`↓` move, `→` opens a card or
unfolds a section, `←` folds the section away, and `r` turns the card reversed.
Inside a card, `→` and `←` step to the next and previous card, and `backspace`
goes back to the outline.

Piped input still works: without a terminal the program falls back to reading
whole lines, so `printf 'j\nq\n' | ./tarot ...` drives it the same way.

### The carousel

`carousel` leaves each card up for a while and then turns the next one over by
itself — in order through the deck, or drawn at random from the pile. Choosing
it from the menu asks which source and how many seconds; `-dwell N` sets the
same thing from the command line, and `-dwell` on `explore` or `freeform` starts
the carousel from those modes directly.

It keeps the wheel in your hands: `space` pauses and resumes, `+` and `-` change
the pace by fifteen seconds, `→` and `←` step forward and back through what you
have seen, and the vi keys scroll a card that runs long. Opening the mindful
reading with `m` stops the clock until you come back.

### Flags

| flag | meaning |
| --- | --- |
| `-mode M` | `celtic`, `three`, `freeform`, `explore`, or `carousel` (default: ask) |
| `-no-reversals` | draw every card upright; no reversed readings |
| `-art S` | `both` (default), `photo` (the deck scan), `sketch` (the guide's line drawing), or `none` |
| `-detail` | always show Waite (1911), rather than waiting for the `w` key |
| `-dwell N` | carousel pace in seconds; on `explore` or `freeform` it starts the carousel |
| `-no-fancy` | print a whole Celtic cross at once: no pictures, no walkthrough, no review |
| `-no-color` | render pictures without ANSI color (also honors `NO_COLOR`) |
| `-height N` | height in rows of the card pictures (default 24) |
| `-quiet` | skip the "Drawing card number N" chatter |
| `-seed N` | fixed seed, for a reproducible reading |
| `-export` | deal a spread straight to `~/tarot_reading_YYYYMMDD_HHMMSS.txt` and exit |
| `-notes PATH` | read the card guide from a markdown file instead of the embedded copy |

Examples:

    ./tarot                                      # pick a mode, then ask your question
    ./tarot -mode three should I take the job
    ./tarot -mode explore                        # browse the deck by section
    ./tarot -mode carousel -dwell 90             # a card every 90 seconds, in order
    ./tarot -mode freeform -dwell 60             # a random card every minute
    ./tarot -mode freeform -no-reversals         # keep pulling, all upright
    ./tarot -no-fancy -no-color what now         # quick text-only Celtic cross
    ./tarot -notes ~/notes/tarot.md -mode three  # read the guide live off disk

## The card guide

`assets/tarot-notes.md` is a copy of `~/notes/tarot.md`, parsed at startup by
`notes.go`. Each of the 78 entries supplies:

* the **imagery**, shown above the card's name,
* the **upright / reversed summary**, matched to how the card landed,
* the **mindful reading**, the long essay behind the `m` key,
* the **Waite (1911)** meanings behind the `w` key,
* the **line drawing**, shown beside the photo (flipped for a reversed card).

The guide's own structure — Major Arcana, then a section per suit — is what
explore mode navigates. The embedded copy keeps the binary self-contained; point
`-notes` at the original to read whatever you have most recently written there.
The parser insists on finding all 78 cards, so a malformed guide fails loudly at
startup rather than producing blank readings.

## Differences from the bash version

* No `jq`, `fzf`, `jp2a`, `awk`, `sed`, or `shuf` — everything is in the binary.
  Raw keyboard input goes through `stty`, which avoids a module dependency for
  terminal handling.
* Images are rendered by `ascii.go` using `image/jpeg`, box-sampling each
  character cell and emitting 24-bit color. Reversed cards are flipped
  vertically, as `jp2a -y` did.
* The `fzf` picker is replaced by a numbered review menu, so there is no `tmp/`
  directory of scratch files: the reading lives in memory.
* The generated "*The Fool in light: the influence that is affecting you
  pertains to...*" sentence is gone, along with the `interpretations.json` it
  drew from. The guide's own summaries say the same thing better.
* Card 21 was `Judgment` in `number_cards.dat` but `Judgement` elsewhere. The
  data is corrected here, and `loadDeck` errors out if the card table and the
  guide ever disagree again.
* Cards come off a shuffled `Pile` dealt without replacement rather than by
  redrawing on collisions, which is what makes free form's endless draw work.

## Data and license

The deck data and images come from bash-the-tarot; the images are the public
domain (in the US) Rider-Waite-Smith deck. The card meanings come from
`~/notes/tarot.md`, which carries contemporary readings alongside A.E. Waite's
originals from *The Pictorial Key to the Tarot* (1911). Like the original, this
is released under CC0; see `LICENSE.md`.
