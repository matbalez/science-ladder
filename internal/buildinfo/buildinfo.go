// Package buildinfo binds official release admission to the compiled source.
package buildinfo

// Commit is injected by the release build. An unstamped development binary
// cannot satisfy an official release attestation for a forty-character commit.
var Commit = "development"
