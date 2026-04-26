// Package buildinfo holds the chcli release version and commit hash.
// Both are populated at link time via:
//
//	-ldflags "-X github.com/vahid-sohrabloo/chcli/internal/buildinfo.Version=v0.2.0
//	          -X github.com/vahid-sohrabloo/chcli/internal/buildinfo.Commit=abc123"
//
// Lives in its own package so non-cmd code (e.g. the status bar) can
// reference it without depending on package main.
package buildinfo

var (
	// Version is the release tag (with or without leading "v"). Defaults
	// to "dev" for unreleased builds.
	Version = "dev"

	// Commit is the short git SHA the binary was built from. Defaults
	// to "none".
	Commit = "none"
)
