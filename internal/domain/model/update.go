package model

// UpdateInfo holds information about a new release.
type UpdateInfo struct {
	CurrentVersion string
	Version        string
	ReleaseURL     string
	DownloadURL    string
}
