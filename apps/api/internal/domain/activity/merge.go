package activity

import "time"

// ActivityMerge retains both versions: Activity is the winning content and
// Revision is the superseded content, even when it arrived after the winner.
// The repository must persist the result and its audit event atomically under
// the same entry lock used by online edits.
type ActivityMerge struct {
	Activity RecordedActivity
	Order    ActivityEditOrder
	Revision ActivityRevision
	Applied  bool
}

func (activity RecordedActivity) MergeEditByOwner(owner string, edit ActivityEdit, currentOrder, incomingOrder ActivityEditOrder, receivedAt time.Time) (ActivityMerge, error) {
	currentOrder, err := NewActivityEditOrder(currentOrder.AuthoredAt, currentOrder.Counter, currentOrder.OperationID)
	if err != nil {
		return ActivityMerge{}, err
	}
	incomingOrder, err = NewActivityEditOrder(incomingOrder.AuthoredAt, incomingOrder.Counter, incomingOrder.OperationID)
	if err != nil {
		return ActivityMerge{}, err
	}
	// Duplicate operations belong to the request-hash/idempotency boundary, not
	// revision creation. Reject ambiguity instead of retaining a second version.
	if incomingOrder.OperationID == currentOrder.OperationID {
		return ActivityMerge{}, errInvalidActivity
	}
	edited, prior, err := activity.EditByOwner(owner, edit, receivedAt)
	if err != nil {
		return ActivityMerge{}, err
	}
	if incomingOrder.Compare(currentOrder) > 0 {
		return ActivityMerge{Activity: edited, Order: incomingOrder, Revision: prior, Applied: true}, nil
	}
	return ActivityMerge{Activity: activity, Order: currentOrder, Revision: ActivityRevision{Activity: edited, ReplacedAt: receivedAt.UTC()}}, nil
}
