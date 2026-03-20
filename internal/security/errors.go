package security

import "errors"

var (
	ErrInvalidToken     = errors.New("invalid token")
	ErrUnauthorized     = errors.New("unauthorized")
	ErrForbidden        = errors.New("forbidden")
	ErrRateLimited      = errors.New("rate limit exceeded")
	ErrBudgetExceeded   = errors.New("token budget exceeded")
	ErrApprovalNotFound = errors.New("approval request not found")
	ErrContentBlocked   = errors.New("content blocked by security filter")
)
