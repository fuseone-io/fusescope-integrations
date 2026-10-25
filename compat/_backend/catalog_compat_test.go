package integrations

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"os"
	"testing"

	"github.com/fuseone-io/fusescope/pkg/query-service/app/integrations/catalog"
)

// Copied into the backend's integrations package by the compat job of
// fuseone-io/fusescope-integrations, once the backend loads the signed catalog;
// FUSESCOPE_INTEGRATION_CATALOG holds the output of `fsint catalog`.
//
// The backend's loader must accept every package, and every package version
// the backend embeds must still be in the catalog: installations pin it.
// `fsint catalog` checks that released versions are unchanged.
func TestCatalogCompat(t *testing.T) {
	dir := os.Getenv("FUSESCOPE_INTEGRATION_CATALOG")
	if dir == "" {
		t.Skip("FUSESCOPE_INTEGRATION_CATALOG is not set")
	}
	fsys := os.DirFS(dir)
	index, err := os.ReadFile(dir + "/" + catalog.IndexFile)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(index)

	candidate, err := catalog.Load(slog.Default(), fsys, hex.EncodeToString(sum[:]), "")
	if err != nil {
		t.Fatalf("the backend rejects the catalog: %v", err)
	}
	if _, err := NewCatalogIntegrations(candidate); err != nil {
		t.Fatalf("the backend rejects the catalog: %v", err)
	}

	embedded, err := catalog.LoadEmbedded(slog.Default(), "")
	if err != nil {
		t.Fatal(err)
	}
	inCandidate := map[string]bool{}
	for _, pkg := range candidate.Packages {
		inCandidate[pkg.ID+"@"+pkg.Version] = true
	}
	for _, pkg := range embedded.Packages {
		if key := pkg.ID + "@" + pkg.Version; !inCandidate[key] {
			t.Errorf("%s is embedded by the backend but missing from the catalog; retire a version only once no backend release supports it", key)
		}
	}
}
