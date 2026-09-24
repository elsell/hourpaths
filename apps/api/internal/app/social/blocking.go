package social

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

const (
	BlockOperation   = "social.block"
	UnblockOperation = "social.unblock"
	blockReviewTTL   = 5 * time.Minute
)

var ErrBlockReviewRequired = errors.New("a current block review acknowledgement is required")

type BlockReviewAcknowledgement struct {
	Version   int
	Token     string
	ExpiresAt time.Time
}

type BlockReview struct {
	Target          domain.BlockTarget
	SharedPaths     []domain.SharedPath
	Acknowledgement BlockReviewAcknowledgement
}

type BlockCommand struct {
	ActorUserID, TargetUsername, TargetUserID string
	ExpectedSharedPathIDs                     []string
	ReviewExpiresAt                           time.Time
	OccurredAt                                time.Time
	Idempotency                               ports.Idempotency
	Audit                                     audit.Event
}

type BlockMutationResult struct {
	Target               domain.BlockTarget
	Blocked              bool
	Changed              bool
	AuthorizationChanges []ports.AuthorizationChange
	Replayed             bool
}

type BlockedAccountPage struct {
	Accounts []domain.BlockedAccount
	HasMore  bool
}

type BlockRepository interface {
	Review(context.Context, string, string) (BlockReview, error)
	Block(context.Context, BlockCommand) (BlockMutationResult, error)
	ListAuthored(context.Context, string, ports.PageRequest) (BlockedAccountPage, error)
	Unblock(context.Context, BlockCommand) (BlockMutationResult, error)
}

func (service *Service) ReviewBlock(ctx context.Context, authorization, rawUsername string) (BlockReview, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return BlockReview{}, err
	}
	username, err := domain.NormalizeUsername(rawUsername)
	if err != nil {
		return BlockReview{}, ports.ErrInvalidArgument
	}
	if err := service.blockReadReady(principal.UserID); err != nil {
		return BlockReview{}, err
	}
	review, err := service.Blocks.Review(ctx, principal.UserID, username)
	if err != nil {
		return BlockReview{}, service.blockReadError(ctx, principal.UserID, err)
	}
	if !validBlockReview(review, principal.UserID) {
		return BlockReview{}, errInvalidDependencies
	}
	review.Acknowledgement, err = service.newBlockReviewAcknowledgement(principal.UserID, review)
	if err != nil {
		return BlockReview{}, err
	}
	event := shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceViewed, "user_block_review", review.Target.UserID, audit.Succeeded)
	if err := service.Audits.AppendAuditEvent(ctx, event); err != nil {
		return BlockReview{}, err
	}
	return review, nil
}

func (service *Service) BlockUser(ctx context.Context, authorization, rawUsername, idempotencyKey string, acknowledgement BlockReviewAcknowledgement) (BlockMutationResult, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return BlockMutationResult{}, err
	}
	username, err := domain.NormalizeUsername(rawUsername)
	if err != nil || !validRelationshipIdempotencyKey(idempotencyKey) {
		return BlockMutationResult{}, ports.ErrInvalidArgument
	}
	claims, err := service.decodeBlockReviewAcknowledgement(acknowledgement.Token)
	if err != nil || acknowledgement.Version != 1 || claims.ActorUserID != principal.UserID || claims.TargetUsername != username {
		return BlockMutationResult{}, ErrBlockReviewRequired
	}
	now, err := service.blockMutationReady(principal.UserID, "profile:"+username)
	if err != nil {
		return BlockMutationResult{}, err
	}
	command := BlockCommand{ActorUserID: principal.UserID, TargetUsername: username, TargetUserID: claims.TargetUserID, ExpectedSharedPathIDs: append([]string(nil), claims.SharedPathIDs...), ReviewExpiresAt: claims.ExpiresAt, OccurredAt: now,
		Idempotency: blockIdempotency(principal.UserID, BlockOperation, idempotencyKey, username+"\x00"+acknowledgement.Token),
		Audit:       shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceCreated, "user_block", username, audit.Succeeded)}
	command.Audit.OccurredAt = now
	result, err := service.Blocks.Block(ctx, command)
	if err != nil {
		return BlockMutationResult{}, service.blockReadError(ctx, principal.UserID, err)
	}
	if !validBlockMutationResult(result, principal.UserID, true) {
		return BlockMutationResult{}, errInvalidDependencies
	}
	if err := service.reconcileBlockAuthorization(ctx, result); err != nil {
		return BlockMutationResult{}, err
	}
	return result, nil
}

type blockReviewClaims struct {
	Version        int       `json:"v"`
	ActorUserID    string    `json:"actor"`
	TargetUserID   string    `json:"targetId"`
	TargetUsername string    `json:"targetUsername"`
	SharedPathIDs  []string  `json:"sharedPathIds"`
	ExpiresAt      time.Time `json:"expiresAt"`
}

func (service *Service) newBlockReviewAcknowledgement(actor string, review BlockReview) (BlockReviewAcknowledgement, error) {
	pathIDs := make([]string, len(review.SharedPaths))
	for index, path := range review.SharedPaths {
		pathIDs[index] = path.ID
	}
	sort.Strings(pathIDs)
	expiresAt := service.Clock.Now().UTC().Add(blockReviewTTL).Truncate(time.Microsecond)
	claims := blockReviewClaims{Version: 1, ActorUserID: actor, TargetUserID: review.Target.UserID, TargetUsername: strings.ToLower(review.Target.Username), SharedPathIDs: pathIDs, ExpiresAt: expiresAt}
	body, err := json.Marshal(claims)
	if err != nil {
		return BlockReviewAcknowledgement{}, err
	}
	mac := hmac.New(sha256.New, service.CursorSigningKey)
	_, _ = mac.Write(body)
	return BlockReviewAcknowledgement{Version: 1, Token: base64.RawURLEncoding.EncodeToString(append(body, mac.Sum(nil)...)), ExpiresAt: expiresAt}, nil
}

func (service *Service) decodeBlockReviewAcknowledgement(value string) (blockReviewClaims, error) {
	var claims blockReviewClaims
	if len(value) == 0 || len(value) > 8192 {
		return claims, ErrBlockReviewRequired
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(raw) <= sha256.Size {
		return claims, ErrBlockReviewRequired
	}
	body, signature := raw[:len(raw)-sha256.Size], raw[len(raw)-sha256.Size:]
	mac := hmac.New(sha256.New, service.CursorSigningKey)
	_, _ = mac.Write(body)
	if !hmac.Equal(signature, mac.Sum(nil)) || json.Unmarshal(body, &claims) != nil || claims.Version != 1 || claims.ActorUserID == "" || claims.TargetUserID == "" || claims.TargetUsername == "" || claims.ExpiresAt.IsZero() || !sort.StringsAreSorted(claims.SharedPathIDs) {
		return blockReviewClaims{}, ErrBlockReviewRequired
	}
	for index, id := range claims.SharedPathIDs {
		if !validOpaqueID(id) || (index > 0 && claims.SharedPathIDs[index-1] == id) {
			return blockReviewClaims{}, ErrBlockReviewRequired
		}
	}
	canonical, err := json.Marshal(claims)
	if err != nil || !hmac.Equal(body, canonical) {
		return blockReviewClaims{}, ErrBlockReviewRequired
	}
	return claims, nil
}

func (service *Service) UnblockUser(ctx context.Context, authorization, targetUserID, idempotencyKey string) (BlockMutationResult, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return BlockMutationResult{}, err
	}
	if !validOpaqueID(targetUserID) || !validRelationshipIdempotencyKey(idempotencyKey) {
		return BlockMutationResult{}, ports.ErrInvalidArgument
	}
	now, err := service.blockMutationReady(principal.UserID, "blocked-account:"+targetUserID)
	if err != nil {
		return BlockMutationResult{}, err
	}
	command := BlockCommand{ActorUserID: principal.UserID, TargetUserID: targetUserID, OccurredAt: now,
		Idempotency: blockIdempotency(principal.UserID, UnblockOperation, idempotencyKey, targetUserID),
		Audit:       shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceDeleted, "user_block", targetUserID, audit.Succeeded)}
	command.Audit.OccurredAt = now
	result, err := service.Blocks.Unblock(ctx, command)
	if err != nil {
		return BlockMutationResult{}, service.blockReadError(ctx, principal.UserID, err)
	}
	if !validBlockMutationResult(result, principal.UserID, false) {
		return BlockMutationResult{}, errInvalidDependencies
	}
	return result, nil
}

func (service *Service) ListBlockedAccounts(ctx context.Context, authorization, cursor string, limit int) ([]domain.BlockedAccount, string, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return nil, "", err
	}
	if limit < 1 || limit > 100 {
		return nil, "", ports.ErrInvalidArgument
	}
	if err := service.blockReadReady(principal.UserID); err != nil {
		return nil, "", err
	}
	now := service.Clock.Now().UTC().Truncate(time.Microsecond)
	pageRequest := ports.PageRequest{Snapshot: now, Limit: limit}
	if cursor != "" {
		claims, decodeErr := decodeBlockedAccountCursor(service.CursorSigningKey, cursor)
		if decodeErr != nil || claims.Viewer != principal.UserID {
			return nil, "", ports.ErrInvalidArgument
		}
		pageRequest.Snapshot, pageRequest.AfterCreated, pageRequest.AfterID = claims.Snapshot, claims.AfterCreated, claims.AfterID
	}
	page, err := service.Blocks.ListAuthored(ctx, principal.UserID, pageRequest)
	if err != nil {
		return nil, "", err
	}
	if !validBlockedAccountPage(page, pageRequest) {
		return nil, "", errInvalidDependencies
	}
	event := shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceListed, "user_block", "authored", audit.Succeeded)
	event.OccurredAt = now
	if err := service.Audits.AppendAuditEvent(ctx, event); err != nil {
		return nil, "", err
	}
	next := ""
	if page.HasMore {
		last := page.Accounts[len(page.Accounts)-1]
		next, err = encodeBlockedAccountCursor(service.CursorSigningKey, blockedAccountCursorClaims{Version: 1, Viewer: principal.UserID, Snapshot: pageRequest.Snapshot, AfterCreated: last.BlockedAt, AfterID: last.Target.UserID})
		if err != nil {
			return nil, "", err
		}
	}
	return page.Accounts, next, nil
}

func (service *Service) blockReadReady(actor string) error {
	if service.Blocks == nil || service.Audits == nil || service.AuditRateLimiter == nil || service.Clock == nil || len(service.CursorSigningKey) < 32 {
		return errInvalidDependencies
	}
	if !service.AuditRateLimiter.Allow(actor, service.Clock.Now().UTC()) {
		return platformapp.ErrRateLimited
	}
	return nil
}

func (service *Service) blockMutationReady(actor, target string) (time.Time, error) {
	if service.Blocks == nil {
		return time.Time{}, errInvalidDependencies
	}
	return service.relationshipMutationReady(actor, target)
}

func (service *Service) blockReadError(ctx context.Context, actor string, repositoryError error) error {
	if !errors.Is(repositoryError, ports.ErrNotFound) {
		return repositoryError
	}
	denial := shared.NewAuditEvent(ctx, service.Clock, actor, actor, audit.ResourceAccessDenied, "user_block", "hidden", audit.Denied)
	if err := service.Audits.AppendAuditEvent(ctx, denial); err != nil {
		return err
	}
	return ports.ErrNotFound
}

func (service *Service) reconcileBlockAuthorization(ctx context.Context, result BlockMutationResult) error {
	for _, change := range result.AuthorizationChanges {
		if err := service.reconcileAuthorizationChange(ctx, change, result.Replayed); err != nil {
			return err
		}
	}
	return nil
}

func validBlockReview(review BlockReview, actor string) bool {
	if !review.Target.Valid() || review.Target.UserID == actor {
		return false
	}
	seen := map[string]struct{}{}
	for _, path := range review.SharedPaths {
		if !path.Valid() {
			return false
		}
		if _, exists := seen[path.ID]; exists {
			return false
		}
		seen[path.ID] = struct{}{}
	}
	return true
}

func validBlockMutationResult(result BlockMutationResult, actor string, blocked bool) bool {
	if !result.Target.Valid() || result.Target.UserID == actor || result.Blocked != blocked || len(result.AuthorizationChanges) > 2 {
		return false
	}
	if !blocked && len(result.AuthorizationChanges) != 0 {
		return false
	}
	seen := map[string]struct{}{}
	for _, change := range result.AuthorizationChanges {
		if change.Operation != ports.AuthorizationDelete || change.Relation != "follower" || change.ResourceType != "user" || change.SubjectType != "user" || change.ActorUserID != actor || change.ID == "" {
			return false
		}
		if !((change.ResourceID == result.Target.UserID && change.SubjectID == actor) || (change.ResourceID == actor && change.SubjectID == result.Target.UserID)) {
			return false
		}
		if _, exists := seen[change.ID]; exists {
			return false
		}
		seen[change.ID] = struct{}{}
	}
	return true
}

func validBlockedAccountPage(page BlockedAccountPage, request ports.PageRequest) bool {
	if len(page.Accounts) > request.Limit || (page.HasMore && len(page.Accounts) == 0) {
		return false
	}
	for index, account := range page.Accounts {
		if !account.Valid() || account.BlockedAt.After(request.Snapshot) {
			return false
		}
		if index > 0 {
			previous := page.Accounts[index-1]
			if account.BlockedAt.After(previous.BlockedAt) || (account.BlockedAt.Equal(previous.BlockedAt) && account.Target.UserID >= previous.Target.UserID) {
				return false
			}
		}
	}
	return true
}

func blockIdempotency(actor, operation, key, target string) ports.Idempotency {
	payload, _ := json.Marshal(struct{ Operation, Target string }{operation, target})
	hash := sha256.Sum256(payload)
	return ports.Idempotency{PrincipalID: actor, Operation: operation, Key: key, RequestHash: hash[:]}
}

type blockedAccountCursorClaims struct {
	Version      int       `json:"v"`
	Viewer       string    `json:"viewer"`
	Snapshot     time.Time `json:"snapshot"`
	AfterCreated time.Time `json:"afterCreated"`
	AfterID      string    `json:"afterId"`
}

func encodeBlockedAccountCursor(key []byte, claims blockedAccountCursorClaims) (string, error) {
	body, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(body)
	return base64.RawURLEncoding.EncodeToString(append(body, mac.Sum(nil)...)), nil
}

func decodeBlockedAccountCursor(key []byte, value string) (blockedAccountCursorClaims, error) {
	var claims blockedAccountCursorClaims
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(raw) <= sha256.Size || len(value) > 4096 {
		return claims, ports.ErrInvalidArgument
	}
	body, signature := raw[:len(raw)-sha256.Size], raw[len(raw)-sha256.Size:]
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(body)
	if !hmac.Equal(signature, mac.Sum(nil)) || json.Unmarshal(body, &claims) != nil || claims.Version != 1 || strings.TrimSpace(claims.Viewer) == "" || claims.Snapshot.IsZero() || claims.AfterCreated.IsZero() || claims.AfterID == "" || claims.AfterCreated.After(claims.Snapshot) {
		return blockedAccountCursorClaims{}, ports.ErrInvalidArgument
	}
	canonical, err := json.Marshal(claims)
	if err != nil || !hmac.Equal(body, canonical) {
		return blockedAccountCursorClaims{}, ports.ErrInvalidArgument
	}
	return claims, nil
}
