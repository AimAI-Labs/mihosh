package model

import "time"

type UnlockStatus string

const (
	UnlockAvailable    UnlockStatus = "available"
	UnlockRestricted   UnlockStatus = "restricted"
	UnlockOriginals    UnlockStatus = "originals"
	UnlockComingSoon   UnlockStatus = "coming_soon"
	UnlockInconclusive UnlockStatus = "inconclusive"
	UnlockFailed       UnlockStatus = "failed"
)

type UnlockResult struct {
	Status    UnlockStatus `json:"status"`
	Region    string       `json:"region,omitempty"`
	CheckedAt time.Time    `json:"checked_at"`
}
