package activity

import "context"

// ExportOwnActivities fixes the participant filter to the authenticated account.
// Existing history authorization, pagination, note visibility and audit rules
// remain authoritative; export is not a new way to read a departed Path.
func (s *Service) ExportOwnActivities(ctx context.Context, authorization, pathID, cursor string, limit int) ([]ActivityListRecord, string, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return nil, "", err
	}
	records, next, err := s.ListActivities(ctx, authorization, pathID, principal.UserID, cursor, limit)
	if err != nil {
		return nil, "", err
	}
	for _, record := range records {
		if record.Activity.ParticipantID != principal.UserID {
			return nil, "", errInvalidDependencies
		}
	}
	return records, next, nil
}
