package social

type Reaction string

const (
	ReactionHeart     Reaction = "heart"
	ReactionApplause  Reaction = "applause"
	ReactionFire      Reaction = "fire"
	ReactionStrong    Reaction = "strong"
	ReactionCelebrate Reaction = "celebrate"
)

var reactions = [...]Reaction{ReactionHeart, ReactionApplause, ReactionFire, ReactionStrong, ReactionCelebrate}

func Reactions() []Reaction {
	result := make([]Reaction, len(reactions))
	copy(result, reactions[:])
	return result
}

func (reaction Reaction) Valid() bool {
	switch reaction {
	case ReactionHeart, ReactionApplause, ReactionFire, ReactionStrong, ReactionCelebrate:
		return true
	default:
		return false
	}
}

type ReactionCounts struct{ Heart, Applause, Fire, Strong, Celebrate int64 }

func (counts ReactionCounts) Valid() bool {
	return counts.Heart >= 0 && counts.Applause >= 0 && counts.Fire >= 0 && counts.Strong >= 0 && counts.Celebrate >= 0
}

type ReactionSummary struct {
	Counts         ReactionCounts
	ViewerReaction Reaction
}

func (summary ReactionSummary) Valid() bool {
	return summary.Counts.Valid() && (summary.ViewerReaction == "" || summary.ViewerReaction.Valid())
}
