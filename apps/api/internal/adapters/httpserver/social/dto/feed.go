package dto

import "time"

type PracticeFeedParticipant struct {
	UserID            string  `json:"userId"`
	Username          string  `json:"username"`
	DisplayName       string  `json:"displayName"`
	ProfilePictureURL *string `json:"profilePictureURL,omitempty"`
}

type PracticeFeedPath struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type PracticeFeedActivity struct {
	ID              string `json:"id"`
	DurationSeconds int64  `json:"durationSeconds" minimum:"1"`
	Edited          bool   `json:"edited"`
}

type GoalAchievement struct {
	Kind              string     `json:"kind" enum:"interval,overall"`
	TargetSeconds     int64      `json:"targetSeconds" minimum:"1"`
	IntervalStartedAt *time.Time `json:"intervalStartedAt,omitempty"`
	IntervalEndedAt   *time.Time `json:"intervalEndedAt,omitempty"`
}

type PracticeReactionCounts struct {
	Heart     int64 `json:"heart" minimum:"0"`
	Applause  int64 `json:"applause" minimum:"0"`
	Fire      int64 `json:"fire" minimum:"0"`
	Strong    int64 `json:"strong" minimum:"0"`
	Celebrate int64 `json:"celebrate" minimum:"0"`
}

type EmojiReaction struct {
	Emoji   string `json:"emoji"`
	Count   int64  `json:"count" minimum:"1"`
	Reacted bool   `json:"reacted"`
}

type PracticeReactionSummary struct {
	EmojiReactions []EmojiReaction        `json:"emojiReactions" nullable:"false"`
	Reactions      PracticeReactionCounts `json:"reactions"`
	ViewerReaction *string                `json:"viewerReaction" enum:"heart,applause,fire,strong,celebrate" nullable:"true"`
}

type PracticeReactionInput struct {
	Reaction string `json:"reaction" enum:"heart,applause,fire,strong,celebrate"`
}

type PracticeFeedItem struct {
	EmojiReactions   []EmojiReaction         `json:"emojiReactions" nullable:"false"`
	ID               string                  `json:"id"`
	Type             string                  `json:"type" enum:"practice_session,goal_achievement"`
	PublishedAt      time.Time               `json:"publishedAt"`
	Participant      PracticeFeedParticipant `json:"participant"`
	Path             PracticeFeedPath        `json:"path"`
	Activity         *PracticeFeedActivity   `json:"activity,omitempty"`
	Achievement      *GoalAchievement        `json:"achievement,omitempty"`
	Reactions        PracticeReactionCounts  `json:"reactions"`
	ViewerReaction   *string                 `json:"viewerReaction" enum:"heart,applause,fire,strong,celebrate" nullable:"true"`
	CommentCount     int64                   `json:"commentCount" minimum:"0"`
	CommentsEnabled  bool                    `json:"commentsEnabled" required:"true"`
	ReactionsEnabled bool                    `json:"reactionsEnabled" required:"true"`
}

type ActivePathIntervalProgress struct {
	RecordedSeconds int64     `json:"recordedSeconds" minimum:"0"`
	TargetSeconds   int64     `json:"targetSeconds" minimum:"1"`
	StartedAt       time.Time `json:"startedAt"`
	EndedAt         time.Time `json:"endedAt"`
	Recurrence      string    `json:"recurrence"`
}
type ActivePathProgress struct {
	AsOf                 time.Time                   `json:"asOf"`
	AccumulatedSeconds   int64                       `json:"accumulatedSeconds" minimum:"0"`
	OverallTargetSeconds *int64                      `json:"overallTargetSeconds,omitempty"`
	Interval             *ActivePathIntervalProgress `json:"interval,omitempty"`
}
type ActiveFollowingTimer struct {
	Progress  *ActivePathProgress `json:"progress,omitempty"`
	ID        string              `json:"id"`
	Path      PracticeFeedPath    `json:"path"`
	StartedAt time.Time           `json:"startedAt"`
}

type ActiveFollowingItem struct {
	Participant PracticeFeedParticipant `json:"participant"`
	Timers      []ActiveFollowingTimer  `json:"timers" nullable:"false"`
}
