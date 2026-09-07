package mysql

import (
	"context"
	"database/sql"
	"errors"

	"github.com/fleetdm/fleet/v4/server/contexts/ctxerr"
	"github.com/fleetdm/fleet/v4/server/fleet"
	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

func (ds *Datastore) UpsertMofaAndroidApp(ctx context.Context, app *fleet.MofaAndroidApp, contents []byte) (*fleet.MofaAndroidApp, error) {
	stmt := `
		INSERT INTO mofa_android_apps
			(team_id, name, package_name, version_name, version_code, filename, sha256, size, contents)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			name = VALUES(name), version_name = VALUES(version_name), filename = VALUES(filename),
			sha256 = VALUES(sha256), size = VALUES(size), contents = VALUES(contents), id = LAST_INSERT_ID(id)`
	res, err := ds.writer(ctx).ExecContext(ctx, stmt, app.TeamID, app.Name, app.PackageName, app.VersionName,
		app.VersionCode, app.Filename, app.SHA256, app.Size, contents)
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1153 {
			return nil, fleet.NewUserMessageError(err, 413)
		}
		return nil, ctxerr.Wrap(ctx, err, "upsert MOFA Android app")
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, ctxerr.Wrap(ctx, err, "get MOFA Android app id")
	}
	return ds.MofaAndroidAppByID(ctx, uint(id), false)
}

func (ds *Datastore) ListMofaAndroidApps(ctx context.Context, teamID uint) ([]*fleet.MofaAndroidApp, error) {
	var apps []*fleet.MofaAndroidApp
	err := sqlx.SelectContext(ctx, ds.reader(ctx), &apps, `
		SELECT id, team_id, name, package_name, version_name, version_code, filename, sha256, size, created_at, updated_at
		FROM mofa_android_apps WHERE team_id = ? ORDER BY name, version_code DESC`, teamID)
	return apps, ctxerr.Wrap(ctx, err, "list MOFA Android apps")
}

func (ds *Datastore) MofaAndroidAppByID(ctx context.Context, appID uint, _ bool) (*fleet.MofaAndroidApp, error) {
	columns := "id, team_id, name, package_name, version_name, version_code, filename, sha256, size, created_at, updated_at"
	app := new(fleet.MofaAndroidApp)
	err := sqlx.GetContext(ctx, ds.reader(ctx), app, "SELECT "+columns+" FROM mofa_android_apps WHERE id = ?", appID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, notFound("MofaAndroidApp").WithID(appID)
	}
	return app, ctxerr.Wrap(ctx, err, "get MOFA Android app")
}

func (ds *Datastore) MofaAndroidHostIDsForTeam(ctx context.Context, teamID uint) ([]uint, error) {
	var hostIDs []uint
	query := "SELECT id FROM hosts WHERE platform = 'android' AND team_id = ? ORDER BY id"
	args := []interface{}{teamID}
	if teamID == 0 {
		query = "SELECT id FROM hosts WHERE platform = 'android' AND team_id IS NULL ORDER BY id"
		args = nil
	}
	if err := sqlx.SelectContext(ctx, ds.reader(ctx), &hostIDs, query, args...); err != nil {
		return nil, ctxerr.Wrap(ctx, err, "list MOFA Android hosts")
	}
	return hostIDs, nil
}

func (ds *Datastore) QueueMofaAndroidAppCommands(ctx context.Context, appID uint, hostIDs []uint) ([]string, error) {
	tx, err := ds.writer(ctx).BeginTxx(ctx, nil)
	if err != nil {
		return nil, ctxerr.Wrap(ctx, err, "begin MOFA Android command transaction")
	}
	defer tx.Rollback()

	commandIDs := make([]string, 0, len(hostIDs))
	for _, hostID := range hostIDs {
		commandID := uuid.NewString()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mofa_android_app_commands (command_id, app_id, host_id, status, detail)
			VALUES (?, ?, ?, 'pending', '')`, commandID, appID, hostID); err != nil {
			return nil, ctxerr.Wrap(ctx, err, "queue MOFA Android app command")
		}
		commandIDs = append(commandIDs, commandID)
	}
	if err := tx.Commit(); err != nil {
		return nil, ctxerr.Wrap(ctx, err, "commit MOFA Android command transaction")
	}
	return commandIDs, nil
}

func (ds *Datastore) ListPendingMofaAndroidAppCommands(ctx context.Context, hostID uint) ([]*fleet.MofaAndroidAppCommand, error) {
	var commands []*fleet.MofaAndroidAppCommand
	err := sqlx.SelectContext(ctx, ds.reader(ctx), &commands, `
		SELECT c.command_id, c.app_id, c.host_id, c.status, c.detail, c.created_at, c.updated_at,
			a.package_name, a.name, a.version_name, a.version_code, a.sha256
		FROM mofa_android_app_commands c
		JOIN mofa_android_apps a ON a.id = c.app_id
		WHERE c.host_id = ? AND c.status IN ('pending', 'downloaded')
		ORDER BY c.created_at`, hostID)
	return commands, ctxerr.Wrap(ctx, err, "list pending MOFA Android app commands")
}

func (ds *Datastore) DownloadMofaAndroidAppForHost(ctx context.Context, hostID uint, commandID string) (*fleet.MofaAndroidAppDownload, error) {
	var download struct {
		Filename string `db:"filename"`
		SHA256   string `db:"sha256"`
		Contents []byte `db:"contents"`
	}
	err := sqlx.GetContext(ctx, ds.reader(ctx), &download, `
		SELECT a.filename, a.sha256, a.contents
		FROM mofa_android_app_commands c
		JOIN mofa_android_apps a ON a.id = c.app_id
		WHERE c.command_id = ? AND c.host_id = ? AND c.status IN ('pending', 'downloaded', 'awaiting_user')`, commandID, hostID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, notFound("MofaAndroidAppCommand").WithName(commandID)
	}
	if err != nil {
		return nil, ctxerr.Wrap(ctx, err, "download MOFA Android app")
	}
	return &fleet.MofaAndroidAppDownload{Filename: download.Filename, SHA256: download.SHA256, Contents: download.Contents}, nil
}

func (ds *Datastore) MofaAndroidAppCommandStatus(ctx context.Context, hostID uint, commandID string) (fleet.MofaAndroidAppCommandStatus, error) {
	var status fleet.MofaAndroidAppCommandStatus
	err := sqlx.GetContext(ctx, ds.reader(ctx), &status,
		`SELECT status FROM mofa_android_app_commands WHERE command_id = ? AND host_id = ?`, commandID, hostID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", notFound("MofaAndroidAppCommand").WithName(commandID)
	}
	return status, ctxerr.Wrap(ctx, err, "get MOFA Android app command status")
}

func (ds *Datastore) UpdateMofaAndroidAppCommandStatus(ctx context.Context, hostID uint, commandID string, status fleet.MofaAndroidAppCommandStatus, detail string) error {
	res, err := ds.writer(ctx).ExecContext(ctx, `
		UPDATE mofa_android_app_commands SET status = ?, detail = ?
		WHERE command_id = ? AND host_id = ?`, status, detail, commandID, hostID)
	if err != nil {
		return ctxerr.Wrap(ctx, err, "update MOFA Android app command status")
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return ctxerr.Wrap(ctx, err, "count updated MOFA Android app command rows")
	}
	if rows == 0 {
		return notFound("MofaAndroidAppCommand").WithName(commandID)
	}
	return nil
}
