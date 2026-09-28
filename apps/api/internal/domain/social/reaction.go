package social

import pathdomain "github.com/elsell/hour-paths/apps/api/internal/domain/path"

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

type EmojiReaction struct {
	Emoji   string
	Count   int64
	Reacted bool
}

type ReactionSummary struct {
	EmojiReactions []EmojiReaction
	Counts         ReactionCounts
	ViewerReaction Reaction
}

func (summary ReactionSummary) Valid() bool {
	for _, value := range summary.EmojiReactions {
		if _, ok := CanonicalEmojiReaction(value.Emoji); !ok || value.Count < 1 {
			return false
		}
	}
	return summary.Counts.Valid() && (summary.ViewerReaction == "" || summary.ViewerReaction.Valid())
}

// Curated aliases remain the storage keys for compatibility with existing clients.
func CanonicalEmojiReaction(emoji string) (Reaction, bool) {
	if !(pathdomain.Appearance{Color: "gold", Emoji: emoji}).ValidChoice() {
		return "", false
	}
	switch emoji {
	case "❤", "❤️":
		return ReactionHeart, true
	case "👏":
		return ReactionApplause, true
	case "🔥":
		return ReactionFire, true
	case "💪":
		return ReactionStrong, true
	case "🎉":
		return ReactionCelebrate, true
	}
	return Reaction(emoji), true
}
func (reaction Reaction) Emoji() string {
	switch reaction {
	case ReactionHeart:
		return "❤️"
	case ReactionApplause:
		return "👏"
	case ReactionFire:
		return "🔥"
	case ReactionStrong:
		return "💪"
	case ReactionCelebrate:
		return "🎉"
	}
	return string(reaction)
}
func (reaction Reaction) ValidStored() bool {
	if reaction.Valid() {
		return true
	}
	value, ok := CanonicalEmojiReaction(string(reaction))
	return ok && value == reaction
}
