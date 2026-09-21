package service

import "errors"

var (
	ErrInvalidTransition = errors.New("requested status transition is not allowed")
	ErrInvalidInput      = errors.New("business input validation failed")
	ErrUnauthorized      = errors.New("invalid username or password")
	ErrInactiveUser      = errors.New("user account is inactive")
	ErrDecisionLocked    = errors.New("final priority decisions are immutable")
	ErrReviewRole        = errors.New("reviewer or admin role is required to finalize a priority")
	ErrSeparationOfDuty  = errors.New("priority preparer cannot approve the same decision")
	ErrNotDecisionOwner  = errors.New("only the preparer may edit this draft decision")
	ErrRetestRole        = errors.New("reviewer or admin role is required to register a retest conclusion")
	ErrRetestSeparation  = errors.New("priority preparer cannot register the retest conclusion")
	ErrRetestNotPending  = errors.New("decision does not await a retest conclusion")
	ErrRetestOverdue     = errors.New("related defect has an overdue retest; register the retest conclusion before finalizing a new decision")
)
