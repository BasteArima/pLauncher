package main

import (
	"testing"

	"game-launcher/internal/models"
)

func TestUpdateDiscoveryDate(t *testing.T) {
	g := &models.Game{Sources: []models.GameSource{{Source: "F95zone", LastVersion: "0.1"}, {Source: "Pornolab", LastVersion: "1"}}}
	check := func(idx int, version string, now, found int64, available bool) {
		t.Helper()
		applySourceVersion(g, idx, version, now)
		if g.UpdateFoundAt != found || g.UpdateAvailable != available || g.LastCheckedAt != now {
			t.Fatalf("version %q: %+v", version, g)
		}
	}
	check(0, "0.2", 100, 100, true)
	check(0, "0.2", 200, 100, true) // A recheck is not a new discovery.
	check(0, "", 300, 100, true)    // Missing metadata preserves the known update.
	check(1, "1", 400, 100, true)   // Another site's baseline cannot clear it.
	check(0, "0.3", 500, 500, true)
	check(1, "2", 600, 600, true)
	check(1, "1", 700, 0, false)
	g.Sources[0].LastVersion = ""
	check(0, "0.4", 800, 0, false) // First check only establishes a baseline.
	g.UpdateAvailable, g.UpdateSource, g.UpdateVersion = true, "F95zone", "0.5"
	check(0, "0.5", 900, 900, true) // Legacy flags get a date on the next discovery.
}
