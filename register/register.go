// Package register exposes the gzip provider to Wago plugin catalogs.
package register

import (
	gzip "github.com/jtenner/wago-gzip"
	wago "github.com/wago-org/wago"
)

// Providers returns the bounded gzip provider with selection-time defaults.
func Providers() []wago.PluginProvider {
	return []wago.PluginProvider{gzip.Provider()}
}
