package gormstore

import (
	"errors"
	"sort"

	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

// lockSocialPairs acquires the same canonical lock used by follow and block
// mutations for every distinct pair. Sorting the normalized pairs keeps
// multi-party mutations (such as comment hearts) free from lock-order cycles.
func lockSocialPairs(tx *gorm.DB, pairs [][2]string) error {
	type pair struct {
		first, second string
		key           string
	}
	unique := make(map[string]pair, len(pairs))
	owners := make([]string, 0, len(pairs)*2)
	for _, candidate := range pairs {
		if candidate[0] == candidate[1] {
			continue
		}
		owners = append(owners, candidate[0], candidate[1])
		ids := []string{candidate[0], candidate[1]}
		sort.Strings(ids)
		key := socialLockKey("social-user-pair", ids[0], ids[1])
		unique[key] = pair{first: ids[0], second: ids[1], key: key}
	}
	if err := lockSocialInteractionOwners(tx, owners); err != nil {
		return err
	}
	ordered := make([]pair, 0, len(unique))
	for _, candidate := range unique {
		ordered = append(ordered, candidate)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].key < ordered[j].key })
	for _, candidate := range ordered {
		if err := lockSocialPair(tx, candidate.first, candidate.second); err != nil {
			return err
		}
	}
	return nil
}

// lockSocialInteractionPairs resolves only the immutable participant IDs
// needed to serialize a mutation. Access and block checks still happen after
// the lock, so a concurrent block always wins before any interaction write.
func lockSocialInteractionPairs(tx *gorm.DB, actor, eventID, commentID string) error {
	var event struct{ OwnerUserID string }
	err := tx.Table("social_feed_event_models").
		Select("participant_user_id AS owner_user_id").
		Where("id = ?", eventID).
		Take(&event).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ports.ErrNotFound
	}
	if err != nil {
		return err
	}

	pairs := [][2]string{{actor, event.OwnerUserID}}
	if commentID != "" {
		var comment struct{ AuthorUserID string }
		err = tx.Table("social_practice_comment_models").
			Select("author_user_id").
			Where("id = ? AND social_feed_event_id = ?", commentID, eventID).
			Take(&comment).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ports.ErrNotFound
		}
		if err != nil {
			return err
		}
		pairs = append(pairs, [2]string{actor, comment.AuthorUserID})
	}
	if err := lockSocialPairs(tx, pairs); err != nil {
		return err
	}
	for _, pair := range pairs {
		if pair[0] == pair[1] {
			continue
		}
		blocked, err := socialPairBlocked(tx, pair[0], pair[1])
		if err != nil {
			return err
		}
		if blocked {
			return ports.ErrNotFound
		}
	}
	return nil
}
