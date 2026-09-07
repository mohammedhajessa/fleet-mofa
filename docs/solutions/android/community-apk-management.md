# Community Android APK management

The MOFA fork adds direct APK distribution for Fleet Community deployments, including phones without Google Mobile Services.

## Security model

- Only a Fleet global administrator can upload or queue APKs.
- The server stores a SHA-256 digest with every APK.
- An authenticated Android host can download only an APK command assigned to that host.
- The agent verifies the digest, package name, and version code before opening Android's installer.
- Installation requires user approval on ordinary Android devices.

This is an app-distribution workflow, not full device-owner management. Remote wipe, kiosk controls, enforced settings, and silent installation require a device-owner/privileged management agent and are outside this Community feature.

## API

| Method | Endpoint | Purpose |
|---|---|---|
| `POST` | `/api/v1/fleet/mofa/android/apps` | Upload an APK and its metadata |
| `GET` | `/api/v1/fleet/mofa/android/apps?team_id=ID` | List uploaded APKs |
| `POST` | `/api/v1/fleet/mofa/android/apps/{id}/install` | Queue installation for `host_ids`, or all Android hosts when empty |
| `GET` | `/api/fleet/orbit/mofa/android/apps/pending` | Agent polls assigned commands |
| `GET` | `/api/fleet/orbit/mofa/android/apps/{command_id}/download` | Agent downloads its assigned APK |
| `POST` | `/api/fleet/orbit/mofa/android/apps/{command_id}/status` | Agent reports command state |

The upload is multipart form data with `software`, `fleet_id`, `name`, `package_name`, `version_name`, and `version_code`.
