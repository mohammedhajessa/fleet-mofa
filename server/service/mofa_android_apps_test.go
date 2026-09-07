package service

import (
	"testing"

	"github.com/fleetdm/fleet/v4/server/fleet"
	"github.com/stretchr/testify/require"
)

func TestValidateMofaAndroidAppUpload(t *testing.T) {
	valid := &fleet.MofaAndroidAppUpload{
		Name:        "Company Portal",
		PackageName: "com.example.portal",
		VersionName: "1.2.3",
		VersionCode: 10203,
		Filename:    "company-portal.apk",
		Contents:    []byte{'P', 'K', 3, 4},
	}
	require.NoError(t, validateMofaAndroidAppUpload(valid))

	tests := []struct {
		name   string
		mutate func(*fleet.MofaAndroidAppUpload)
	}{
		{"missing name", func(upload *fleet.MofaAndroidAppUpload) { upload.Name = "" }},
		{"invalid package", func(upload *fleet.MofaAndroidAppUpload) { upload.PackageName = "not a package" }},
		{"missing version name", func(upload *fleet.MofaAndroidAppUpload) { upload.VersionName = "" }},
		{"zero version code", func(upload *fleet.MofaAndroidAppUpload) { upload.VersionCode = 0 }},
		{"wrong extension", func(upload *fleet.MofaAndroidAppUpload) { upload.Filename = "portal.zip" }},
		{"not a zip archive", func(upload *fleet.MofaAndroidAppUpload) { upload.Contents = []byte("not an apk") }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upload := *valid
			tt.mutate(&upload)
			require.Error(t, validateMofaAndroidAppUpload(&upload))
		})
	}
	require.Error(t, validateMofaAndroidAppUpload(nil))
}

func TestValidMofaAndroidAppStatusTransition(t *testing.T) {
	require.True(t, validMofaAndroidAppStatusTransition(fleet.MofaAndroidAppCommandPending, fleet.MofaAndroidAppCommandDownloaded))
	require.True(t, validMofaAndroidAppStatusTransition(fleet.MofaAndroidAppCommandDownloaded, fleet.MofaAndroidAppCommandAwaitingUser))
	require.True(t, validMofaAndroidAppStatusTransition(fleet.MofaAndroidAppCommandAwaitingUser, fleet.MofaAndroidAppCommandInstalled))
	require.True(t, validMofaAndroidAppStatusTransition(fleet.MofaAndroidAppCommandPending, fleet.MofaAndroidAppCommandFailed))
	require.False(t, validMofaAndroidAppStatusTransition(fleet.MofaAndroidAppCommandPending, fleet.MofaAndroidAppCommandInstalled))
	require.False(t, validMofaAndroidAppStatusTransition(fleet.MofaAndroidAppCommandInstalled, fleet.MofaAndroidAppCommandPending))
}
