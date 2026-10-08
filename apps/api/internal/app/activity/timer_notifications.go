package activity

import (
	"context"
	"strings"
)

func (s *Service) timerNotificationRecipients(ctx context.Context, actor, path string) ([]string, error) {
	if s.Repository == nil || s.Authorizer == nil {
		return nil, errInvalidDependencies
	}
	candidates, err := s.Repository.TimerNotificationCandidates(ctx, actor, path)
	if err != nil {
		return nil, err
	}
	recipients := make([]string, 0, len(candidates))
	seen := make(map[string]bool, len(candidates))
	for _, recipient := range candidates {
		if strings.TrimSpace(recipient) == "" || strings.TrimSpace(recipient) != recipient || recipient == actor || seen[recipient] {
			return nil, errInvalidDependencies
		}
		seen[recipient] = true
		allowed, err := s.Authorizer.Check(ctx, "path", path, "view", recipient)
		if err != nil {
			return nil, err
		}
		if allowed {
			recipients = append(recipients, recipient)
		}
	}
	return recipients, nil
}
