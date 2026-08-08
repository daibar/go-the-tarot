package main

// PositionDef describes one slot in a spread: what it is called and what it
// signifies.
type PositionDef struct {
	Name    string
	Meaning string
}

// Spread is a named layout. Freeform and explore modes use no spread at all.
type Spread struct {
	Key       string
	Title     string
	Blurb     string
	Positions []PositionDef
}

func (s *Spread) Len() int {
	if s == nil {
		return 0
	}
	return len(s.Positions)
}

// celticSpread is the ten card Celtic cross, the layout the bash version drew.
var celticSpread = &Spread{
	Key:   "celtic",
	Title: "Celtic cross",
	Blurb: "ten cards, the full spread",
	Positions: []PositionDef{
		{
			Name:    "Card 1: The Present or The Self",
			Meaning: "The Present or The Self - This position illustrates the present circumstances and ongoing events. It can also paint a picture of your current mental state and provide a glimpse of you identity at the present moment.",
		},
		{
			Name:    "Card 2: The Problem",
			Meaning: "The Problem - This card symbolizes the hurdles that you are grappling with, issues which need resolving to move ahead.",
		},
		{
			Name:    "Card 3: The Past",
			Meaning: "The Past - This position gives an insight into past occurrences and their contribution to the contemporary situation. It offers clues about the past influencers that steered the present conditions.",
		},
		{
			Name:    "Card 4: The Future",
			Meaning: "The Future - This card predicts possible developments, assuming the status quo. These are generally short-term predictions and do not depict an ultimate conclusion.",
		},
		{
			Name:    "Card 5: Conscious",
			Meaning: "Conscious - This card scrutinizes what has your attention and where your thoughts lie. It could signify your objectives and wishes concerning this circumstance, as well as your expectations.",
		},
		{
			Name:    "Card 6: Unconscious",
			Meaning: "Unconscious - This card reveals the concealed underlying causes of the situation; the emotions, principles and values that the querent might still be clueless about. This card can sometimes be unexpected and signify a hidden determinant.",
		},
		{
			Name:    "Card 7: Your Influence",
			Meaning: "Your Influence - This card can be interpreted in various ways - though overall, it's about how one's self-image can potentially shape outcomes. It asks what convictions about yourself are you carrying? Are you expanding or restricting yourself?",
		},
		{
			Name:    "Card 8: External Influence",
			Meaning: "External Influence - This card portrays the external world and its impact on the situation. It might symbolize the societal and emotional milieu, along with others' perceptions.",
		},
		{
			Name:    "Card 9: Hopes and Fears",
			Meaning: "Hopes and Fears - This is a complex position to decipher, as the card can signify both: clandestine wishes and potential avoidances. It reflects the paradoxical human nature, where what we dread the most might be exactly what we have been yearning for.",
		},
		{
			Name:    "Card 10: Outcome",
			Meaning: "Outcome - This card acts as an aggregate of the preceding cards. With everything considered, what is the probable outcome of the event? If the interpretation here does not suggest a positive result, the remaining cards in the spread can help find an alternative path.",
		},
	},
}

// threeSpread is the classic past / present / future reading.
var threeSpread = &Spread{
	Key:   "three",
	Title: "Three card",
	Blurb: "past, present, future",
	Positions: []PositionDef{
		{
			Name:    "Card 1: The Past",
			Meaning: "The Past - What has already happened and still shapes the question: the history, the choices, and the conditions you are carrying into the present.",
		},
		{
			Name:    "Card 2: The Present",
			Meaning: "The Present - Where the matter stands today: the energy at work right now, what has your attention, and what is being asked of you in this moment.",
		},
		{
			Name:    "Card 3: The Future",
			Meaning: "The Future - Where the current course leads. Not a verdict but a trajectory: what unfolds if the present conditions are allowed to run on.",
		},
	},
}

var spreads = map[string]*Spread{
	celticSpread.Key: celticSpread,
	threeSpread.Key:  threeSpread,
}
