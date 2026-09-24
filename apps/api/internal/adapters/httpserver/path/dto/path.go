package dto

import "time"

import activitydto "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/activity/dto"

type PathItem struct {
	ID            string            `json:"id"`
	Name          string            `json:"name" required:"true" minLength:"1"`
	Visibility    string            `json:"visibility" required:"true" enum:"private,followers,public"`
	IntervalGoal  *IntervalGoalItem `json:"intervalGoal,omitempty"`
	OverallTarget *OverallTarget    `json:"overallTarget,omitempty"`
	ArchivedAt    *time.Time        `json:"archivedAt,omitempty"`
}

type PathCapabilities struct {
	TrackTime         bool `json:"trackTime" required:"true"`
	RenamePath        bool `json:"renamePath" required:"true"`
	InviteMembers     bool `json:"inviteMembers" required:"true"`
	ManageMembers     bool `json:"manageMembers" required:"true"`
	ManageGoals       bool `json:"manageGoals" required:"true"`
	ManageLifecycle   bool `json:"manageLifecycle" required:"true"`
	ManageVisibility  bool `json:"manageVisibility" required:"true"`
	TransferOwnership bool `json:"transferOwnership" required:"true"`
	LeavePath         bool `json:"leavePath" required:"true"`
}

type PathProjection struct {
	ID            string            `json:"id"`
	Name          string            `json:"name" required:"true" minLength:"1"`
	Visibility    string            `json:"visibility" required:"true" enum:"private,followers,public"`
	IntervalGoal  *IntervalGoalItem `json:"intervalGoal,omitempty"`
	OverallTarget *OverallTarget    `json:"overallTarget,omitempty"`
	ArchivedAt    *time.Time        `json:"archivedAt,omitempty"`
	Capabilities  PathCapabilities  `json:"capabilities" required:"true"`
	Home          *HomeOrganization `json:"home,omitempty"`
}

type HomeOrganization struct {
	Classification   string     `json:"classification" required:"true" enum:"solo,shared,supporting"`
	Pinned           bool       `json:"pinned" required:"true"`
	PinnedPosition   *int64     `json:"pinnedPosition,omitempty" minimum:"0"`
	ManualPosition   *int64     `json:"manualPosition,omitempty" minimum:"0"`
	RecentActivityAt *time.Time `json:"recentActivityAt,omitempty"`
}

type HomePreferences struct {
	OrderMethod   string     `json:"orderMethod" required:"true" enum:"recent,alphabetical,manual"`
	Revision      int64      `json:"revision" required:"true" minimum:"0"`
	PinnedPathIDs []string   `json:"pinnedPathIds" required:"true" nullable:"false"`
	ManualPathIDs []string   `json:"manualPathIds" required:"true" nullable:"false"`
	UpdatedAt     *time.Time `json:"updatedAt,omitempty"`
}

type HomePreferencesUpdate struct {
	ExpectedRevision int64    `json:"expectedRevision" required:"true" minimum:"0"`
	OrderMethod      string   `json:"orderMethod" required:"true" enum:"recent,alphabetical,manual"`
	PinnedPathIDs    []string `json:"pinnedPathIds" required:"true" nullable:"false"`
	ManualPathIDs    []string `json:"manualPathIds" required:"true" nullable:"false"`
}

type PathCreate struct {
	Name          string              `json:"name" required:"true" minLength:"1"`
	Visibility    string              `json:"visibility,omitempty" enum:"private,followers,public"`
	IntervalGoal  *IntervalGoalCreate `json:"intervalGoal,omitempty"`
	OverallTarget *OverallTarget      `json:"overallTarget,omitempty"`
}

type IntervalGoalCreate struct {
	TargetSeconds int64              `json:"targetSeconds" required:"true" minimum:"1"`
	Recurrence    string             `json:"recurrence" required:"true" enum:"hourly,daily,weekly,monthly,yearly"`
	Alignment     *IntervalAlignment `json:"alignment,omitempty"`
}

type IntervalGoalItem struct {
	TargetSeconds int64             `json:"targetSeconds" required:"true" minimum:"1"`
	Recurrence    string            `json:"recurrence" required:"true" enum:"hourly,daily,weekly,monthly,yearly"`
	Alignment     IntervalAlignment `json:"alignment" required:"true"`
}

type OverallTarget struct {
	TargetSeconds int64 `json:"targetSeconds" required:"true" minimum:"1"`
}

type IntervalAlignment struct {
	Minute     *int `json:"minute,omitempty" minimum:"0" maximum:"59"`
	Hour       *int `json:"hour,omitempty" minimum:"0" maximum:"23"`
	ISOWeekday *int `json:"isoWeekday,omitempty" minimum:"1" maximum:"7"`
	Month      *int `json:"month,omitempty" minimum:"1" maximum:"12"`
	Day        *int `json:"day,omitempty" minimum:"1" maximum:"31"`
}

type PathUpdate struct {
	Name       string `json:"name" required:"true" minLength:"1"`
	Visibility string `json:"visibility" required:"true" minLength:"1"`
}

type PathRename struct {
	ExpectedName string `json:"expectedName" required:"true" minLength:"1" maxLength:"100"`
	Name         string `json:"name" required:"true" minLength:"1" maxLength:"100"`
}

type PathVisibilityUpdate struct {
	Confirmed          bool   `json:"confirmed" required:"true"`
	ExpectedVisibility string `json:"expectedVisibility" required:"true" enum:"private,followers,public"`
	Visibility         string `json:"visibility" required:"true" enum:"private,followers,public"`
}

type PathLeave struct {
	Confirmed      bool `json:"confirmed" required:"true"`
	RetainActivity bool `json:"retainActivity" required:"true"`
}

type PathLeaveReceipt struct {
	PathID           string `json:"pathId" required:"true"`
	Left             bool   `json:"left" required:"true"`
	ActivityRetained bool   `json:"activityRetained" required:"true"`
}

type MemberRemovalReview struct {
	UserID              string `json:"userId" required:"true"`
	Username            string `json:"username" required:"true"`
	DisplayName         string `json:"displayName" required:"true"`
	Role                string `json:"role" enum:"participant,supporter"`
	SessionCount        int64  `json:"sessionCount" minimum:"0"`
	TotalTrackedSeconds int64  `json:"totalTrackedSeconds" minimum:"0"`
	RunningTimer        bool   `json:"runningTimer"`
}
type PathMember struct {
	UserID                   string        `json:"userId" required:"true"`
	Username                 string        `json:"username" required:"true"`
	DisplayName              string        `json:"displayName" required:"true"`
	Role                     string        `json:"role" required:"true" enum:"creator,administrator,participant,supporter"`
	SessionCount             int64         `json:"sessionCount" minimum:"0"`
	TotalTrackedSeconds      int64         `json:"totalTrackedSeconds" minimum:"0"`
	IntervalProgress         *GoalProgress `json:"intervalProgress,omitempty"`
	OverallProgress          *GoalProgress `json:"overallProgress,omitempty"`
	BlockedByViewer          bool          `json:"blockedByViewer"`
	CanRemove                bool          `json:"canRemove"`
	CanChangeRole            bool          `json:"canChangeRole"`
	CanGrantAdministrator    bool          `json:"canGrantAdministrator"`
	CanRevokeAdministrator   bool          `json:"canRevokeAdministrator"`
	CanStepDownAdministrator bool          `json:"canStepDownAdministrator"`
	CanLeave                 bool          `json:"canLeave"`
}

type GoalProgress struct {
	AccumulatedSeconds int64 `json:"accumulatedSeconds" minimum:"0"`
	TargetSeconds      int64 `json:"targetSeconds" minimum:"1"`
}

type MemberRemoval struct {
	Confirmed    bool   `json:"confirmed" required:"true"`
	ExpectedRole string `json:"expectedRole" required:"true" enum:"participant,supporter"`
}

type MemberRemovalReceipt struct {
	PathID          string `json:"pathId" required:"true"`
	UserID          string `json:"userId" required:"true"`
	Removed         bool   `json:"removed" required:"true"`
	ActivityDeleted bool   `json:"activityDeleted" required:"true"`
}

type MemberRoleChange struct {
	Confirmed    bool   `json:"confirmed" required:"true"`
	ExpectedRole string `json:"expectedRole" required:"true" enum:"participant,supporter,administrator"`
	Role         string `json:"role" required:"true" enum:"participant,supporter,administrator"`
}

type MemberRoleChangeReceipt struct {
	PathID          string `json:"pathId" required:"true"`
	UserID          string `json:"userId" required:"true"`
	Role            string `json:"role" required:"true" enum:"participant,supporter,administrator"`
	ActivityDeleted bool   `json:"activityDeleted" required:"true"`
}

type PathGoalsUpdate struct {
	Confirmed     bool                  `json:"confirmed" required:"true"`
	ExpectedGoals PathGoalConfiguration `json:"expectedGoals" required:"true"`
	IntervalGoal  *IntervalGoalItem     `json:"intervalGoal,omitempty"`
	OverallTarget *OverallTarget        `json:"overallTarget,omitempty"`
}

type PathGoalConfiguration struct {
	IntervalGoal  *IntervalGoalItem `json:"intervalGoal,omitempty"`
	OverallTarget *OverallTarget    `json:"overallTarget,omitempty"`
}

type PathGoalMutationResult struct {
	Path               PathProjection                `json:"path"`
	AccumulatedSeconds int64                         `json:"accumulatedSeconds" minimum:"0"`
	IntervalProgress   *activitydto.IntervalProgress `json:"intervalProgress,omitempty"`
}

type PathArchiveStateUpdate struct {
	Confirmed        bool `json:"confirmed" required:"true"`
	ExpectedArchived bool `json:"expectedArchived" required:"true"`
	Archived         bool `json:"archived" required:"true"`
}

type PathDelete struct {
	Confirmed    bool   `json:"confirmed" required:"true"`
	ExpectedName string `json:"expectedName" required:"true" minLength:"1" maxLength:"100"`
}

type PathDeletionReceipt struct {
	PathID  string `json:"pathId" required:"true"`
	Deleted bool   `json:"deleted" required:"true"`
}
