// Package dashboard provides an embedded filesystem for serving
// the wotp dashboard static files. The React dashboard is built
// separately and its output placed in dashboard/dist/ before
// the Go binary is compiled.
package dashboard

import "embed"

// DistFS embeds the dashboard/dist/ directory containing the built
// React application. When building without the dashboard, the
// placeholder index.html is used instead.
//
//go:embed dist/*
var DistFS embed.FS
