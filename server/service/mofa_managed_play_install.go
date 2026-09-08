package service

import (
	"context"
	"regexp"

	"github.com/fleetdm/fleet/v4/server/fleet"
	androidmanagement "google.golang.org/api/androidmanagement/v1"
)

var mofaAndroidPackageName = regexp.MustCompile(
	`^([A-Za-z][A-Za-z0-9_]*\.)+[A-Za-z][A-Za-z0-9_]*$`,
)

type installMofaManagedPlayAppRequest struct {
	HostUUID    string `json:"host_uuid"`
	PackageName string `json:"package_name"`
}

type installMofaManagedPlayAppResponse struct {
	Name        string `json:"name,omitempty"`
	PackageName string `json:"package_name,omitempty"`
	HostUUID    string `json:"host_uuid,omitempty"`
	Status      string `json:"status,omitempty"`
	Err         error  `json:"error,omitempty"`
}

func (r installMofaManagedPlayAppResponse) Error() error {
	return r.Err
}

func installMofaManagedPlayAppEndpoint(
	ctx context.Context,
	request interface{},
	svc fleet.Service,
) (fleet.Errorer, error) {
	req := request.(*installMofaManagedPlayAppRequest)
	name, err := svc.InstallMofaManagedPlayApp(
		ctx,
		req.HostUUID,
		req.PackageName,
	)
	if err != nil {
		return &installMofaManagedPlayAppResponse{Err: err}, nil
	}

	return &installMofaManagedPlayAppResponse{
		Name:        name,
		PackageName: req.PackageName,
		HostUUID:    req.HostUUID,
		Status:      "policy_updated",
	}, nil
}

func (svc *Service) InstallMofaManagedPlayApp(
	ctx context.Context,
	hostUUID string,
	packageName string,
) (string, error) {
	hosts, err := svc.authorizeAllHostsTeams(
		ctx,
		[]string{hostUUID},
		fleet.ActionWrite,
		&fleet.MDMCommandAuthz{},
	)
	if err != nil {
		return "", err
	}

	if len(hosts) != 1 {
		return "", fleet.NewInvalidArgumentError(
			"host_uuid",
			"Android host was not found",
		)
	}

	if hosts[0].Platform != string(fleet.AndroidPlatform) {
		return "", fleet.NewInvalidArgumentError(
			"host_uuid",
			"The selected host is not Android",
		)
	}

	if !mofaAndroidPackageName.MatchString(packageName) {
		return "", fleet.NewInvalidArgumentError(
			"package_name",
			"Invalid Android package name",
		)
	}

	enterprise, err := svc.ds.GetEnterprise(ctx)
	if err != nil {
		return "", &fleet.BadRequestError{
			Message:     "Android MDM is not enabled",
			InternalErr: err,
		}
	}

	app, err := svc.androidSvc.EnterprisesApplications(
		ctx,
		enterprise.Name(),
		packageName,
	)
	if err != nil {
		return "", fleet.NewInvalidArgumentError(
			"package_name",
			"Application was not found in managed Google Play",
		)
	}

	policy := &androidmanagement.ApplicationPolicy{
		PackageName:             packageName,
		InstallType:             "FORCE_INSTALLED",
		AutoUpdateMode:          "AUTO_UPDATE_HIGH_PRIORITY",
		DefaultPermissionPolicy: "GRANT",
	}

	_, err = svc.androidSvc.AddAppsToAndroidPolicy(
		ctx,
		enterprise.Name(),
		[]*androidmanagement.ApplicationPolicy{policy},
		map[string]string{hostUUID: hostUUID},
	)
	if err != nil {
		return "", err
	}

	return app.Title, nil
}
