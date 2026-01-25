package names

import (
	"math/rand"
	"time"
)

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

// Culture ship name components in the style of Iain M. Banks
var (
	prefixes = []string{
		"A", "The", "No", "Just", "Only", "Of Course I Still",
		"So Much For", "What Are The Alarm Bells For",
		"Very Little", "Absolutely No", "Whose", "Someone Else's",
		"I Thought He Was With You", "Mistake Not",
		"You'll Thank Me Later", "Quietly Confident",
		"Experiencing A Conditions", "Zero",
		"Unfortunate", "Unqualified", "Unreliable",
		"Conditions", "Whose Concern", "Whose Idea Was This",
	}

	cores = []string{
		"Gravitas", "Ambition", "Attitude", "Problem",
		"Regret", "Doubt", "Ethics", "Morality",
		"Patience", "Virtue", "Subtlety", "Restraint",
		"Enthusiasm", "Optimism", "Irony", "Context",
		"Margin", "Error", "Signal", "Noise",
		"Intention", "Consequence", "Coincidence", "Certainty",
		"Assumption", "Assertion", "Negotiation", "Position",
		"Perspective", "Priority", "Protocol", "Procedure",
		"Opportunity", "Objection", "Observation", "Opinion",
	}

	suffixes = []string{
		"Shortfall", "Surplus", "Deficit", "Excess",
		"Supply", "Gradient", "Differential", "Quotient",
		"Threshold", "Boundary", "Horizon", "Tangent",
		"Vector", "Trajectory", "Variable", "Constant",
		"Resonance", "Frequency", "Wavelength", "Amplitude",
		"", "", "", "", // empty for variety
	}

	standalone = []string{
		"Conditions Of Satisfaction",
		"Conditions Of Uncertainty",
		"Conditions Under Which Progress Seems Possible",
		"Conditions Prevailing In The Annoying Announcers' Box",
		"Conditions Permitting",
		"Different Tan",
		"Dramatic Exit Only",
		"Experiencing A Conditions",
		"Frank Exchange Of Views",
		"Gunboat Diplomat",
		"Honest Mistake",
		"I Blame The Parents",
		"I Blame Your Mother",
		"I Said I Had A Plan",
		"Irregular Apocalypse",
		"It's Character Forming",
		"Just Read The Instructions",
		"Just Testing",
		"Lacking In Concern",
		"Lacking Concern For Whose Benefit",
		"Lapsed Pacifist",
		"Learned Response",
		"Legitimate Salvage",
		"Limiting Factor",
		"Lightly Seared On The Reality Grill",
		"Me I'm Counting",
		"Misophist",
		"Mistake Not My Current State Of Conditions For Alarm",
		"No Fixed Abode",
		"No More Alarm Bells",
		"Not Invented Here",
		"Now Look What You Made Me Do",
		"Now We Try It My Way",
		"Of Course I Told You This Already",
		"Outcome Not Guaranteed",
		"Outside Context Problem",
		"Passing By And Thought I'd Drop In",
		"Poke It With A Stick",
		"Conditions Favoring Regret",
		"Conditions Favoring Uncertainty",
		"Conditions Favoring Excessive Caution",
		"Conditions Favoring Inappropriate Response",
		"Conditions Favoring Victory",
		"Conditions Unfavorable",
		"Conditions Uncertain",
		"Quietly Confident",
		"Reasonable Excuse",
		"Conditions Prevailing",
		"Conditions Normal All Alarm Bells Ringing",
		"Reformed Nice Guy",
		"Conditions General",
		"Conditions Local",
		"Conditions Elsewhere",
		"Conditions Present",
		"Relative Calm",
		"Conditions Relative",
		"Conditions Optimal For Error",
		"Conditions Suboptimal",
		"Conditions Unspecified",
		"Conditions Specified",
		"Conditions Known",
		"Conditions Unknown",
		"Conditions Changing",
		"Conditions Changed",
		"Conditions Stable",
		"Conditions Unstable",
		"Conditions Transient",
		"Conditions Permanent",
		"Conditions Temporary",
		"Resistance Is Impolite",
		"Conditions Improving",
		"Conditions Deteriorating",
		"Sanctioned Parts List",
		"Conditions Manageable",
		"Conditions Unmanageable",
		"Conditions Resolved",
		"Conditions Unresolved",
		"Serious Callers Only",
		"Size Isn't Everything",
		"Sleeper Service",
		"So Much For Conditions",
		"So Much For Subtlety",
		"Someone Should Tell Them",
		"Steely Glint",
		"Stranger Here Myself",
		"System Conditions",
		"Conditions Within Normal Parameters",
		"Tactical Grace",
		"Thank You For Your Input",
		"That's Still Not Conditions",
		"The Ends Of Conditions",
		"Conditions At The Edge",
		"Conditions In The Middle",
		"Conditions Everywhere",
		"Conditions Nowhere",
		"Thinking About It",
		"Conditions Under Review",
		"Conditions Pending",
		"Unfortunate Conditions In Transit",
		"Uninvited Guest",
		"Very Little Conditions Supply",
		"What Are The Alarm Bells For",
		"What Conditions",
		"Whose Conditions Is This Anyway",
		"Wisdom Like Silence",
		"Youthful Indiscretion",
		"Zero Conditions",
		"Conditions Of Whose Concern",
		"Conditions Of Whose Convenience",
	}
)

// Generate returns a random Culture ship-style name
func Generate() string {
	// 60% chance of standalone name, 40% chance of constructed name
	if rng.Float32() < 0.6 {
		return standalone[rng.Intn(len(standalone))]
	}

	prefix := prefixes[rng.Intn(len(prefixes))]
	core := cores[rng.Intn(len(cores))]
	suffix := suffixes[rng.Intn(len(suffixes))]

	if suffix == "" {
		return prefix + " " + core
	}
	return prefix + " " + core + " " + suffix
}
