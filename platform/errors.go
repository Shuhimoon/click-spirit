package platform

import "errors"

// ErrUnsupported is returned by New on platforms without a desktop clicker.
// Android apps should call the mobile package instead of platform.New.
var ErrUnsupported = errors.New("platform clicker is not supported on this OS; Android users should use the mobile package")
