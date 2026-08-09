package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type options struct {
	art     artStyle
	color   bool
	height  int
	detail  bool
	quiet   bool
	noFancy bool
	bare    bool          // just the picture: no drawing, no words
	layout  bool          // open a spread on its tableau
	cols    int           // terminal width the page is laid out for
	rows    int           // terminal height, for sizing the tableau
	dwell   time.Duration // carousel pace; 0 means wait for a keypress
}

func main() {
	var (
		modeFlag    = flag.String("mode", "", "reading to do: celtic, three, freeform, explore, or carousel (default: ask)")
		artFlag     = flag.String("art", string(artBoth), "card pictures: both, photo, sketch, or none")
		reversals   = flag.Bool("reversals", true, "let cards come up reversed")
		noReversals = flag.Bool("no-reversals", false, "draw every card upright (the same as -reversals=false)")
		noFancy     = flag.Bool("no-fancy", false, "print the whole reading at once: no pictures, no walkthrough, no review")
		noColor     = flag.Bool("no-color", false, "render pictures without ANSI color")
		detail      = flag.Bool("detail", false, "always show the Waite (1911) meanings, not just on the w key")
		quiet       = flag.Bool("quiet", false, "skip the \"drawing card N\" chatter")
		height      = flag.Int("height", 0, "height in rows of the card pictures (0 = fit the terminal)")
		layout      = flag.Bool("layout", false, "open a spread on its tableau and skip the card by card walkthrough")
		random      = flag.Bool("random", false, "carousel: draw at random from the pile rather than walking the deck in order")
		dwell       = flag.Int("dwell", 0, "carousel: seconds each card stays up before the next turns over (0 = turn them by hand)")
		seed        = flag.Int64("seed", 0, "fixed random seed, for reproducible readings (0 = derive from query and clock)")
		export      = flag.Bool("export", false, "write the reading to ~/tarot_reading_DATE.txt and exit")
		notesPath   = flag.String("notes", "", "path to a card guide markdown file (default: the embedded copy of tarot.md)")
	)
	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintf(out, "usage: %s [flags] [your question here]\n\nflags:\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
		fmt.Fprintf(out, "\nmodes:\n")
		for _, m := range modeMenu {
			fmt.Fprintf(out, "  %-10s %s\n", m.mode, m.blurb)
		}
		fmt.Fprintf(out, `
Every mode and option can be reached from the command line, without the menu:

  %[1]s -mode celtic -no-reversals        upright ten card spread
  %[1]s -mode three -layout               three cards, straight to the tableau
  %[1]s -mode freeform -reversals         keep drawing, reversals on
  %[1]s -mode carousel -dwell 90          a card every 90 seconds, in order
  %[1]s -mode carousel -random -dwell 30  ... drawn at random instead
  %[1]s -mode explore -art sketch         browse the deck as line drawings
`, filepath.Base(os.Args[0]))
	}
	flag.Parse()

	if err := run(*modeFlag, options{
		art:     artStyle(*artFlag),
		color:   !*noColor && os.Getenv("NO_COLOR") == "",
		height:  *height,
		detail:  *detail,
		quiet:   *quiet,
		noFancy: *noFancy,
		dwell:   time.Duration(*dwell) * time.Second,
		layout:  *layout,
	}, runFlags{
		query:       strings.Join(flag.Args(), " "),
		reversals:   *reversals && !*noReversals,
		random:      *random,
		seed:        *seed,
		export:      *export,
		notesPath:   *notesPath,
		noFancyMode: *noFancy,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "tarot:", err)
		os.Exit(1)
	}
}

type runFlags struct {
	query       string
	reversals   bool
	random      bool
	seed        int64
	export      bool
	notesPath   string
	noFancyMode bool
}

func run(modeFlag string, opts options, f runFlags) error {
	switch opts.art {
	case artBoth, artPhoto, artSketch, artNone:
	default:
		return fmt.Errorf("unknown -art %q: want both, photo, sketch, or none", opts.art)
	}
	if opts.noFancy {
		opts.art = artNone
	}

	deck, err := loadDeck(f.notesPath)
	if err != nil {
		return err
	}
	if opts.noFancy || f.export {
		if opts.height <= 0 {
			opts.height = maxArtHeight
		}
	}
	rng := rand.New(rand.NewSource(seedFrom(f.seed, f.query)))
	pile := newPile(deck, rng, f.reversals)

	// The runs that never stop for input stay out of raw mode entirely.
	if f.export || opts.noFancy {
		s := spreads[string(mode(strings.ToLower(modeFlag)))]
		if s == nil {
			if modeFlag != "" && mode(strings.ToLower(modeFlag)) != modeCeltic {
				return fmt.Errorf("%s needs a terminal: use one of %s here", modeFlag, spreadNames())
			}
			s = celticSpread
		}
		progress := func(n int) { fmt.Printf("Drawing card number %d\n", n) }
		if opts.quiet || f.export {
			progress = nil
		}
		r := newReading(deck, pile, s, f.query, progress)
		if !f.export {
			fmt.Print(plain(r, opts))
			return nil
		}
		path, err := exportReading(r, opts)
		if err != nil {
			return err
		}
		fmt.Println("Reading written to", path)
		return nil
	}

	t := newTerm()
	defer t.close()
	restoreOnSignal(t)

	// Lay the page out for the terminal we actually have, so a card has a
	// chance of fitting on one screen.
	opts.cols, opts.rows = t.cols, t.rows
	if opts.height <= 0 {
		opts.height = min(max(t.rows-6, 8), maxArtHeight)
	}

	base := opts
	u := &ui{t: t, deck: deck, pile: pile, opts: opts, query: f.query}

	p, err := pickMode(u, modeFlag, f)
	if err != nil {
		return err
	}
	// Quitting out of a mode lands back here at the menu, rather than ending
	// the program; only quitting the menu itself does that.
	for p.mode != "" {
		if p.reversals != nil {
			pile.reversals = *p.reversals
		}
		if p.query != nil {
			u.query = *p.query
		}
		runMode(u, p, base)

		var ok bool
		p, ok = chooseMode(u)
		if !ok {
			return nil // the reader quit at the menu
		}
	}
	return nil
}

// runMode dispatches to the chosen mode's loop, starting from the base options
// so a previous mode's toggles (bare, detail) don't leak into the next.
func runMode(u *ui, p plan, base options) {
	u.opts = base
	switch p.mode {
	case modeCarousel:
		u.opts.dwell = p.dwell
		u.opts.detail = true
		runCarousel(u, p.ordered)
	case modeFreeform:
		runFreeform(u)
	case modeExplore:
		u.opts.detail = true // browsing the deck is what the notes are for
		runExplore(u)
	default:
		runSpread(u, spreads[string(p.mode)])
	}
}

// restoreOnSignal puts the terminal back the way it was if we are killed.
func restoreOnSignal(t *term) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		t.close()
		os.Exit(130)
	}()
}

// pickMode resolves -mode, or asks when the flag is absent. A dwell time turns
// explore and freeform into the carousel, which is what the flag combination
// means: the same cards, turning over on their own.
func pickMode(u *ui, modeFlag string, f runFlags) (plan, error) {
	if modeFlag == "" {
		p, ok := chooseMode(u)
		if !ok {
			return plan{}, nil
		}
		return p, nil
	}

	m := mode(strings.ToLower(modeFlag))
	p := plan{mode: m, ordered: !f.random, dwell: u.opts.dwell}
	if _, ok := spreads[string(m)]; ok {
		return p, nil
	}
	switch m {
	case modeExplore:
		if p.dwell > 0 {
			p.mode = modeCarousel
		}
		return p, nil
	case modeFreeform:
		if p.dwell > 0 {
			p.mode, p.ordered = modeCarousel, false
		}
		return p, nil
	case modeCarousel:
		if p.dwell <= 0 {
			p.dwell = defaultDwell
		}
		return p, nil
	}
	return plan{}, fmt.Errorf("unknown -mode %q: want %s, freeform, explore, or carousel", modeFlag, spreadNames())
}

// seedFrom mixes the query string into the clock, as the bash version did with
// od(1). An explicit -seed overrides both so a reading can be reproduced.
func seedFrom(explicit int64, query string) int64 {
	if explicit != 0 {
		return explicit
	}
	seed := time.Now().Unix()
	for _, b := range []byte(query) {
		seed += int64(b)
	}
	return seed
}

func exportReading(r *Reading, opts options) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	// Exports carry the full guide material; there is no keyboard in a file.
	opts.detail = true
	body := []byte(plain(r, opts))

	// Two readings in the same second must not overwrite one another.
	base := filepath.Join(home, "tarot_reading_"+time.Now().Format("20060102_150405"))
	for n := 1; ; n++ {
		path := base + ".txt"
		if n > 1 {
			path = fmt.Sprintf("%s_%d.txt", base, n)
		}
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		_, writeErr := file.Write(body)
		closeErr := file.Close()
		if writeErr != nil {
			return "", writeErr
		}
		return path, closeErr
	}
}
