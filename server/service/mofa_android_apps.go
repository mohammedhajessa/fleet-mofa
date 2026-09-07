package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	hostctx "github.com/fleetdm/fleet/v4/server/contexts/host"
	"github.com/fleetdm/fleet/v4/server/fleet"
)

const maxMofaAPKSize = 200 * 1024 * 1024

var androidPackageNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*(\.[A-Za-z][A-Za-z0-9_]*)+$`)

type mofaAndroidAppDatastore interface {
	UpsertMofaAndroidApp(ctx context.Context, app *fleet.MofaAndroidApp, contents []byte) (*fleet.MofaAndroidApp, error)
	ListMofaAndroidApps(ctx context.Context, teamID uint) ([]*fleet.MofaAndroidApp, error)
	MofaAndroidAppByID(ctx context.Context, appID uint, includeContents bool) (*fleet.MofaAndroidApp, error)
	MofaAndroidHostIDsForTeam(ctx context.Context, teamID uint) ([]uint, error)
	QueueMofaAndroidAppCommands(ctx context.Context, appID uint, hostIDs []uint) ([]string, error)
	ListPendingMofaAndroidAppCommands(ctx context.Context, hostID uint) ([]*fleet.MofaAndroidAppCommand, error)
	DownloadMofaAndroidAppForHost(ctx context.Context, hostID uint, commandID string) (*fleet.MofaAndroidAppDownload, error)
	MofaAndroidAppCommandStatus(ctx context.Context, hostID uint, commandID string) (fleet.MofaAndroidAppCommandStatus, error)
	UpdateMofaAndroidAppCommandStatus(ctx context.Context, hostID uint, commandID string, status fleet.MofaAndroidAppCommandStatus, detail string) error
}

func (svc *Service) mofaAndroidStore() (mofaAndroidAppDatastore, error) {
	store, ok := svc.ds.(mofaAndroidAppDatastore)
	if !ok {
		return nil, errors.New("MOFA Android app datastore is not configured")
	}
	return store, nil
}

func (svc *Service) UploadMofaAndroidApp(ctx context.Context, upload *fleet.MofaAndroidAppUpload) (*fleet.MofaAndroidApp, error) {
	if err := svc.authz.Authorize(ctx, &fleet.AppConfig{}, fleet.ActionWrite); err != nil {
		return nil, err
	}
	if err := validateMofaAndroidAppUpload(upload); err != nil {
		return nil, err
	}
	digest := sha256.Sum256(upload.Contents)
	app := &fleet.MofaAndroidApp{
		TeamID:      upload.TeamID,
		Name:        upload.Name,
		PackageName: upload.PackageName,
		VersionName: upload.VersionName,
		VersionCode: upload.VersionCode,
		Filename:    filepath.Base(upload.Filename),
		SHA256:      hex.EncodeToString(digest[:]),
		Size:        int64(len(upload.Contents)),
	}
	store, err := svc.mofaAndroidStore()
	if err != nil {
		return nil, err
	}
	return store.UpsertMofaAndroidApp(ctx, app, upload.Contents)
}

func validateMofaAndroidAppUpload(upload *fleet.MofaAndroidAppUpload) error {
	if upload == nil {
		return &fleet.BadRequestError{Message: "APK upload is required"}
	}
	if strings.TrimSpace(upload.Name) == "" {
		return &fleet.BadRequestError{Message: "app name is required"}
	}
	if !androidPackageNamePattern.MatchString(upload.PackageName) {
		return &fleet.BadRequestError{Message: "package_name must be a valid Android application ID"}
	}
	if strings.TrimSpace(upload.VersionName) == "" || upload.VersionCode == 0 {
		return &fleet.BadRequestError{Message: "version_name and a positive version_code are required"}
	}
	if !strings.EqualFold(filepath.Ext(upload.Filename), ".apk") {
		return &fleet.BadRequestError{Message: "software must be an .apk file"}
	}
	if len(upload.Contents) < 4 || string(upload.Contents[:2]) != "PK" {
		return &fleet.BadRequestError{Message: "uploaded file is not a valid APK archive"}
	}
	if len(upload.Contents) > maxMofaAPKSize {
		return &fleet.BadRequestError{Message: "APK exceeds the 200 MB limit"}
	}
	return nil
}

func (svc *Service) ListMofaAndroidApps(ctx context.Context, teamID uint) ([]*fleet.MofaAndroidApp, error) {
	if err := svc.authz.Authorize(ctx, &fleet.AppConfig{}, fleet.ActionRead); err != nil {
		return nil, err
	}
	store, err := svc.mofaAndroidStore()
	if err != nil {
		return nil, err
	}
	return store.ListMofaAndroidApps(ctx, teamID)
}

func (svc *Service) QueueMofaAndroidAppInstall(ctx context.Context, appID uint, hostIDs []uint) ([]string, error) {
	if err := svc.authz.Authorize(ctx, &fleet.AppConfig{}, fleet.ActionWrite); err != nil {
		return nil, err
	}
	store, err := svc.mofaAndroidStore()
	if err != nil {
		return nil, err
	}
	app, err := store.MofaAndroidAppByID(ctx, appID, false)
	if err != nil {
		return nil, err
	}
	if len(hostIDs) == 0 {
		hostIDs, err = store.MofaAndroidHostIDsForTeam(ctx, app.TeamID)
		if err != nil {
			return nil, err
		}
		if len(hostIDs) == 0 {
			return nil, &fleet.BadRequestError{Message: "no enrolled Android hosts were found in this fleet"}
		}
	}
	for _, hostID := range hostIDs {
		host, err := svc.ds.Host(ctx, hostID)
		if err != nil {
			return nil, err
		}
		if host.FleetPlatform() != string(fleet.AndroidPlatform) {
			return nil, &fleet.BadRequestError{Message: fmt.Sprintf("host %d is not an Android host", hostID)}
		}
		hostTeamID := uint(0)
		if host.TeamID != nil {
			hostTeamID = *host.TeamID
		}
		if hostTeamID != app.TeamID {
			return nil, &fleet.BadRequestError{Message: fmt.Sprintf("app %d is not available to host %d's fleet", appID, hostID)}
		}
	}
	return store.QueueMofaAndroidAppCommands(ctx, appID, hostIDs)
}

func (svc *Service) ListPendingMofaAndroidAppCommands(ctx context.Context) ([]*fleet.MofaAndroidAppCommand, error) {
	svc.authz.SkipAuthorization(ctx)
	host, ok := hostctx.FromContext(ctx)
	if !ok {
		return nil, errors.New("missing authenticated Android host")
	}
	store, err := svc.mofaAndroidStore()
	if err != nil {
		return nil, err
	}
	return store.ListPendingMofaAndroidAppCommands(ctx, host.ID)
}

func (svc *Service) DownloadMofaAndroidApp(ctx context.Context, commandID string) (*fleet.MofaAndroidAppDownload, error) {
	svc.authz.SkipAuthorization(ctx)
	host, ok := hostctx.FromContext(ctx)
	if !ok {
		return nil, errors.New("missing authenticated Android host")
	}
	store, err := svc.mofaAndroidStore()
	if err != nil {
		return nil, err
	}
	return store.DownloadMofaAndroidAppForHost(ctx, host.ID, commandID)
}

func (svc *Service) UpdateMofaAndroidAppCommandStatus(ctx context.Context, commandID string, status fleet.MofaAndroidAppCommandStatus, detail string) error {
	svc.authz.SkipAuthorization(ctx)
	host, ok := hostctx.FromContext(ctx)
	if !ok {
		return errors.New("missing authenticated Android host")
	}
	store, err := svc.mofaAndroidStore()
	if err != nil {
		return err
	}
	current, err := store.MofaAndroidAppCommandStatus(ctx, host.ID, commandID)
	if err != nil {
		return err
	}
	if !validMofaAndroidAppStatusTransition(current, status) {
		return &fleet.BadRequestError{Message: fmt.Sprintf("invalid APK command status transition from %s to %s", current, status)}
	}
	if len(detail) > 4_096 {
		detail = detail[:4_096]
	}
	return store.UpdateMofaAndroidAppCommandStatus(ctx, host.ID, commandID, status, detail)
}

func validMofaAndroidAppStatusTransition(from, to fleet.MofaAndroidAppCommandStatus) bool {
	if from == to {
		return true
	}
	if to == fleet.MofaAndroidAppCommandFailed {
		return from == fleet.MofaAndroidAppCommandPending || from == fleet.MofaAndroidAppCommandDownloaded || from == fleet.MofaAndroidAppCommandAwaitingUser
	}
	switch from {
	case fleet.MofaAndroidAppCommandPending:
		return to == fleet.MofaAndroidAppCommandDownloaded
	case fleet.MofaAndroidAppCommandDownloaded:
		return to == fleet.MofaAndroidAppCommandAwaitingUser
	case fleet.MofaAndroidAppCommandAwaitingUser:
		return to == fleet.MofaAndroidAppCommandInstalled
	default:
		return false
	}
}

type uploadMofaAndroidAppRequest struct {
	Upload *fleet.MofaAndroidAppUpload
}

func (uploadMofaAndroidAppRequest) DecodeRequest(_ context.Context, r *http.Request) (interface{}, error) {
	r.Body = http.MaxBytesReader(nil, r.Body, maxMofaAPKSize+1024*1024)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return nil, &fleet.BadRequestError{Message: "failed to parse APK upload: " + err.Error()}
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	files := r.MultipartForm.File["software"]
	if len(files) != 1 {
		return nil, &fleet.BadRequestError{Message: "software multipart field must contain one APK"}
	}
	file, err := files[0].Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()
	contents, err := io.ReadAll(io.LimitReader(file, maxMofaAPKSize+1))
	if err != nil {
		return nil, err
	}
	versionCode, err := strconv.ParseUint(r.FormValue("version_code"), 10, 64)
	if err != nil {
		return nil, &fleet.BadRequestError{Message: "version_code must be a positive integer"}
	}
	teamID := uint64(0)
	if rawTeamID := r.FormValue("fleet_id"); rawTeamID != "" {
		teamID, err = strconv.ParseUint(rawTeamID, 10, 32)
		if err != nil {
			return nil, &fleet.BadRequestError{Message: "fleet_id must be an integer"}
		}
	}
	return &uploadMofaAndroidAppRequest{Upload: &fleet.MofaAndroidAppUpload{
		TeamID:      uint(teamID),
		Name:        strings.TrimSpace(r.FormValue("name")),
		PackageName: strings.TrimSpace(r.FormValue("package_name")),
		VersionName: strings.TrimSpace(r.FormValue("version_name")),
		VersionCode: versionCode,
		Filename:    files[0].Filename,
		Contents:    contents,
	}}, nil
}

type uploadMofaAndroidAppResponse struct {
	App *fleet.MofaAndroidApp `json:"app,omitempty"`
	Err error                 `json:"error,omitempty"`
}

func (r uploadMofaAndroidAppResponse) Error() error { return r.Err }

func uploadMofaAndroidAppEndpoint(ctx context.Context, request interface{}, svc fleet.Service) (fleet.Errorer, error) {
	req := request.(*uploadMofaAndroidAppRequest)
	app, err := svc.UploadMofaAndroidApp(ctx, req.Upload)
	return uploadMofaAndroidAppResponse{App: app, Err: err}, nil
}

type listMofaAndroidAppsRequest struct {
	TeamID uint `query:"team_id,optional" renameto:"fleet_id"`
}

type listMofaAndroidAppsResponse struct {
	Apps []*fleet.MofaAndroidApp `json:"apps"`
	Err  error                   `json:"error,omitempty"`
}

func (r listMofaAndroidAppsResponse) Error() error { return r.Err }

func listMofaAndroidAppsEndpoint(ctx context.Context, request interface{}, svc fleet.Service) (fleet.Errorer, error) {
	req := request.(*listMofaAndroidAppsRequest)
	apps, err := svc.ListMofaAndroidApps(ctx, req.TeamID)
	return listMofaAndroidAppsResponse{Apps: apps, Err: err}, nil
}

type queueMofaAndroidAppRequest struct {
	AppID   uint   `url:"app_id"`
	HostIDs []uint `json:"host_ids"`
}

type queueMofaAndroidAppResponse struct {
	CommandIDs []string `json:"command_ids,omitempty"`
	Err        error    `json:"error,omitempty"`
}

func (r queueMofaAndroidAppResponse) Error() error { return r.Err }
func (r queueMofaAndroidAppResponse) Status() int  { return http.StatusAccepted }

func queueMofaAndroidAppEndpoint(ctx context.Context, request interface{}, svc fleet.Service) (fleet.Errorer, error) {
	req := request.(*queueMofaAndroidAppRequest)
	ids, err := svc.QueueMofaAndroidAppInstall(ctx, req.AppID, req.HostIDs)
	return queueMofaAndroidAppResponse{CommandIDs: ids, Err: err}, nil
}

type pendingMofaAndroidAppsRequest struct{}

type pendingMofaAndroidAppsResponse struct {
	Commands []*fleet.MofaAndroidAppCommand `json:"commands"`
	Err      error                          `json:"error,omitempty"`
}

func (r pendingMofaAndroidAppsResponse) Error() error { return r.Err }

func pendingMofaAndroidAppsEndpoint(ctx context.Context, _ interface{}, svc fleet.Service) (fleet.Errorer, error) {
	commands, err := svc.ListPendingMofaAndroidAppCommands(ctx)
	return pendingMofaAndroidAppsResponse{Commands: commands, Err: err}, nil
}

type downloadMofaAndroidAppRequest struct {
	CommandID string `url:"command_id"`
}

type downloadMofaAndroidAppResponse struct {
	Download *fleet.MofaAndroidAppDownload
	Err      error `json:"error,omitempty"`
}

func (r downloadMofaAndroidAppResponse) Error() error { return r.Err }

func (r downloadMofaAndroidAppResponse) HijackRender(_ context.Context, w http.ResponseWriter) {
	if r.Err != nil || r.Download == nil {
		return
	}
	w.Header().Set("Content-Type", "application/vnd.android.package-archive")
	w.Header().Set("Content-Length", strconv.Itoa(len(r.Download.Contents)))
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{
		"filename": filepath.Base(r.Download.Filename),
	}))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-APK-SHA256", r.Download.SHA256)
	_, _ = w.Write(r.Download.Contents)
}

func downloadMofaAndroidAppEndpoint(ctx context.Context, request interface{}, svc fleet.Service) (fleet.Errorer, error) {
	req := request.(*downloadMofaAndroidAppRequest)
	download, err := svc.DownloadMofaAndroidApp(ctx, req.CommandID)
	return downloadMofaAndroidAppResponse{Download: download, Err: err}, nil
}

type updateMofaAndroidAppStatusRequest struct {
	CommandID string                            `url:"command_id"`
	Status    fleet.MofaAndroidAppCommandStatus `json:"status"`
	Detail    string                            `json:"detail"`
}

type updateMofaAndroidAppStatusResponse struct {
	Err error `json:"error,omitempty"`
}

func (r updateMofaAndroidAppStatusResponse) Error() error { return r.Err }

func updateMofaAndroidAppStatusEndpoint(ctx context.Context, request interface{}, svc fleet.Service) (fleet.Errorer, error) {
	req := request.(*updateMofaAndroidAppStatusRequest)
	err := svc.UpdateMofaAndroidAppCommandStatus(ctx, req.CommandID, req.Status, req.Detail)
	return updateMofaAndroidAppStatusResponse{Err: err}, nil
}
