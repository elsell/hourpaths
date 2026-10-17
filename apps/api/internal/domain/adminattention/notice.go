package adminattention

import "time"

type Notice struct {
	ID, Kind, TargetID string
	CreatedAt          time.Time
	Read               bool
}
type Record struct {
	ID, Kind, Reason, Explanation, State string
	CreatedAt                            time.Time
}
