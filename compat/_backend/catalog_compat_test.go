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
// The backend's loader must accept every package, and a package version the
// backend already embeds must render to the same document: released versions
// are immutable.
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
	released := map[string]string{}
	for _, pkg := range embedded.Packages {
		released[pkg.ID+"@"+pkg.Version] = string(pkg.Document)
	}
	for _, pkg := range candidate.Packages {
		key := pkg.ID + "@" + pkg.Version
		document, ok := released[key]
		if !ok {
			t.Logf("%s is not in the backend's snapshot yet", key)
			continue
		}
		if document != string(pkg.Document) {
			t.Errorf("%s differs from the released package the backend embeds; bump its version", key)
		}
	}
}
