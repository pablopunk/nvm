package emoji

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// updateDataset rewrites the shipped dataset from the catalog library. Run it after the catalog changes.
var updateDataset = flag.Bool("update-dataset", false, "rewrite internal/ext/bundled/datasets/emoji-symbols.json")

// datasetFile is the shipped file the picker reads, relative to this package.
var datasetFile = filepath.Join("..", "..", "bundled", "datasets", DatasetName+".json")

// TestShippedDatasetMatchesTheCatalog checks that the file the app ships holds the catalog's records, as the library
// builds them. A catalog change without a dataset update fails here.
func TestShippedDatasetMatchesTheCatalog(t *testing.T) {
	want, err := json.MarshalIndent(Dataset(), "", " ")
	if err != nil {
		t.Fatal(err)
	}
	want = append(want, '\n')
	if *updateDataset {
		if err := os.WriteFile(datasetFile, want, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(datasetFile)
	if err != nil {
		t.Fatalf("read the shipped dataset: %v (run with -update-dataset)", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("the shipped dataset is out of date; run with -update-dataset")
	}
}
