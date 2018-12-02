package hypro

import (
	"github.com/blang/semver"
	"github.com/pkg/errors"
)

const (
	// Version is the current hypro version
	Version = "0.1.0"
	// MinClientVersion is the current hypro proto version
	MinClientVersion = "0.1.0"
)

var (
	serverVersion, _    = semver.Make(Version)
	minClientVersion, _ = semver.Make(MinClientVersion)
)

// checkVersionCompatible checks if the client's protocol is compatible
func checkVersionCompatible(clientVersion string) (bool, error) {
	v, err := semver.Make(clientVersion)
	if err != nil {
		return false, errors.Wrap(err, "version incorrect")
	}
	return v.GTE(minClientVersion) && v.LTE(serverVersion), nil
}
