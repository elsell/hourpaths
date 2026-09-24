package path

import "time"

const ownershipTransferCandidateCursorDomain = "path-ownership-transfer-candidate"

// OwnershipTransferCandidatePageRequest defines a stable account-creation
// snapshot and keyset. Candidate eligibility is still revalidated separately
// when a transfer is initiated.
type OwnershipTransferCandidatePageRequest struct {
	AfterUserID  string
	AfterCreated time.Time
	Snapshot     time.Time
	Limit        int
}

type OwnershipTransferCandidatePage struct {
	Items   []OwnershipTransferCandidate
	HasMore bool
}
