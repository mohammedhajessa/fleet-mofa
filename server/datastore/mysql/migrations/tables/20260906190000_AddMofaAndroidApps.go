package tables

import (
	"database/sql"
	"fmt"
)

func init() {
	MigrationClient.AddMigration(Up_20260906190000, Down_20260906190000)
}

func Up_20260906190000(tx *sql.Tx) error {
	if _, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS mofa_android_apps (
			id INT UNSIGNED NOT NULL AUTO_INCREMENT,
			team_id INT UNSIGNED NOT NULL DEFAULT 0,
			name VARCHAR(255) NOT NULL,
			package_name VARCHAR(255) NOT NULL,
			version_name VARCHAR(255) NOT NULL,
			version_code BIGINT UNSIGNED NOT NULL,
			filename VARCHAR(255) NOT NULL,
			sha256 CHAR(64) COLLATE utf8mb4_bin NOT NULL,
			size BIGINT NOT NULL,
			contents LONGBLOB NOT NULL,
			created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
			updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
			PRIMARY KEY (id),
			UNIQUE KEY idx_mofa_android_apps_version (team_id, package_name, version_code),
			KEY idx_mofa_android_apps_package (package_name)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
	`); err != nil {
		return fmt.Errorf("create mofa_android_apps table: %w", err)
	}

	if _, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS mofa_android_app_commands (
			command_id CHAR(36) COLLATE utf8mb4_bin NOT NULL,
			app_id INT UNSIGNED NOT NULL,
			host_id INT UNSIGNED NOT NULL,
			status VARCHAR(32) NOT NULL DEFAULT 'pending',
			detail TEXT NOT NULL,
			created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
			updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
			PRIMARY KEY (command_id),
			KEY idx_mofa_android_commands_host_status (host_id, status),
			CONSTRAINT fk_mofa_android_commands_app FOREIGN KEY (app_id) REFERENCES mofa_android_apps (id) ON DELETE CASCADE,
			CONSTRAINT fk_mofa_android_commands_host FOREIGN KEY (host_id) REFERENCES hosts (id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
	`); err != nil {
		return fmt.Errorf("create mofa_android_app_commands table: %w", err)
	}
	return nil
}

func Down_20260906190000(tx *sql.Tx) error {
	return nil
}
