package notification

type Channel string

func Channels() []Channel {
	return []Channel{"following", "path_access", "tracking_activity", "achievements", "comments", "reactions", "comment_hearts", "nudges", "goal_reminders", "timer_health"}
}
func (channel Channel) Valid() bool {
	for _, candidate := range Channels() {
		if candidate == channel {
			return true
		}
	}
	return false
}
