package service

import "errors"

var (
	ErrInvalidTransition       = errors.New("requested status transition is not allowed")
	ErrInvalidInput            = errors.New("business input validation failed")
	ErrUnauthorized            = errors.New("invalid username or password")
	ErrInactiveUser            = errors.New("user account is inactive")
	ErrDecisionLocked          = errors.New("final priority decisions are immutable")
	ErrReviewRole              = errors.New("reviewer or admin role is required to finalize a priority")
	ErrSeparationOfDuty        = errors.New("priority preparer cannot approve the same decision")
	ErrNotDecisionOwner        = errors.New("only the preparer may edit this draft decision")
	ErrDefectRetestOverdue     = errors.New("defect has an overdue retest; register the retest conclusion before a new finalization")
	ErrRetestNotRequired       = errors.New("retest conclusion only applies to finalized restrict or urgent decisions")
	ErrRetestAlreadyRegistered = errors.New("retest conclusion is already registered for this decision")
)
