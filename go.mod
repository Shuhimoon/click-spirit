module github.com/click-spirit/click-spirit

// golang.org/x/sys v0.49.0 declares go 1.26.0. A bare "go 1.26" makes
// GOOS=windows builds stop and ask for `go mod tidy`.
go 1.26.0

require golang.org/x/sys v0.49.0
