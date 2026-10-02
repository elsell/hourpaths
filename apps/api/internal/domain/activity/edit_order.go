package activity

import (
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const maximumEditCounter int64 = 9007199254740991

// ActivityEditOrder orders authored versions independently of delivery time.
// Receipt time still governs persistence and audit timestamps.
type ActivityEditOrder struct {
	AuthoredAt  time.Time
	Counter     int64
	OperationID string
}

func NewActivityEditOrder(authoredAt time.Time, counter int64, operationID string) (ActivityEditOrder, error) {
	if authoredAt.IsZero() || authoredAt.Year() < 1 || authoredAt.Year() > 9999 || counter < 0 || counter > maximumEditCounter || operationID == "" || len(operationID) > 128 || !utf8.ValidString(operationID) || strings.IndexFunc(operationID, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
		return ActivityEditOrder{}, errors.New("invalid activity edit order")
	}
	return ActivityEditOrder{AuthoredAt: authoredAt.UTC().Truncate(time.Microsecond), Counter: counter, OperationID: operationID}, nil
}

func NextActivityEditOrder(now time.Time, previous ActivityEditOrder, operationID string) (ActivityEditOrder, error) {
	if now.IsZero() {
		return ActivityEditOrder{}, errors.New("invalid activity edit clock")
	}
	if _, err := NewActivityEditOrder(previous.AuthoredAt, previous.Counter, previous.OperationID); err != nil {
		return ActivityEditOrder{}, err
	}
	now = now.UTC().Truncate(time.Microsecond)
	if now.After(previous.AuthoredAt) {
		return NewActivityEditOrder(now, 0, operationID)
	}
	return NewActivityEditOrder(previous.AuthoredAt, previous.Counter+1, operationID)
}

func (order ActivityEditOrder) Compare(other ActivityEditOrder) int {
	if result := order.AuthoredAt.Compare(other.AuthoredAt); result != 0 {
		return result
	}
	if order.Counter < other.Counter {
		return -1
	}
	if order.Counter > other.Counter {
		return 1
	}
	return strings.Compare(order.OperationID, other.OperationID)
}
