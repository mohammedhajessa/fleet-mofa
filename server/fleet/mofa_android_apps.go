package fleet

import (
	"context"
	"time"
)

// MofaAndroidApp is an APK uploaded directly to Fleet. It is intentionally
// separate from Managed Google Play applications so Community deployments can
// manage devices that do not include Google services.
type MofaAndroidApp struct {
	ID          uint      `json:"id" db:"id"`
	TeamID      uint      `json:"team_id" renameto:"fleet_id" db:"team_id"`
	Name        string    `json:"name" db:"name"`
	PackageName string    `json:"package_name" db:"package_name"`
	VersionName string    `json:"version_name" db:"version_name"`
	VersionCode uint64    `json:"version_code" db:"version_code"`
	Filename    string    `json:"filename" db:"filename"`
	SHA256      string    `json:"sha256" db:"sha256"`
	Size        int64     `json:"size" db:"size"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

type MofaAndroidAppUpload struct {
	TeamID      uint
	Name        string
	PackageName string
	VersionName string
	VersionCode uint64
	Filename    string
	Contents    []byte
}

type MofaAndroidAppCommandStatus string

const (
	MofaAndroidAppCommandPending      MofaAndroidAppCommandStatus = "pending"
	MofaAndroidAppCommandDownloaded   MofaAndroidAppCommandStatus = "downloaded"
	MofaAndroidAppCommandAwaitingUser MofaAndroidAppCommandStatus = "awaiting_user"
	MofaAndroidAppCommandInstalled    MofaAndroidAppCommandStatus = "installed"
	MofaAndroidAppCommandFailed       MofaAndroidAppCommandStatus = "failed"
)

type MofaAndroidAppCommand struct {
	CommandID   string                      `json:"command_id" db:"command_id"`
	AppID       uint                        `json:"app_id" db:"app_id"`
	HostID      uint                        `json:"host_id,omitempty" db:"host_id"`
	PackageName string                      `json:"package_name" db:"package_name"`
	Name        string                      `json:"name" db:"name"`
	VersionName string                      `json:"version_name" db:"version_name"`
	VersionCode uint64                      `json:"version_code" db:"version_code"`
	SHA256      string                      `json:"sha256" db:"sha256"`
	Status      MofaAndroidAppCommandStatus `json:"status,omitempty" db:"status"`
	Detail      string                      `json:"detail,omitempty" db:"detail"`
	CreatedAt   time.Time                   `json:"created_at,omitempty" db:"created_at"`
	UpdatedAt   time.Time                   `json:"updated_at,omitempty" db:"updated_at"`
}

type MofaAndroidAppDownload struct {
	Filename string
	SHA256   string
	Contents []byte
}

// MofaAndroidAppService is kept separate from Fleet's Premium software
// package APIs. These methods are implemented by the Community service.
type MofaAndroidAppService interface {
	UploadMofaAndroidApp(ctx context.Context, upload *MofaAndroidAppUpload) (*MofaAndroidApp, error)
	ListMofaAndroidApps(ctx context.Context, teamID uint) ([]*MofaAndroidApp, error)
	QueueMofaAndroidAppInstall(ctx context.Context, appID uint, hostIDs []uint) ([]string, error)
	ListPendingMofaAndroidAppCommands(ctx context.Context) ([]*MofaAndroidAppCommand, error)
	DownloadMofaAndroidApp(ctx context.Context, commandID string) (*MofaAndroidAppDownload, error)
	UpdateMofaAndroidAppCommandStatus(ctx context.Context, commandID string, status MofaAndroidAppCommandStatus, detail string) error
}
