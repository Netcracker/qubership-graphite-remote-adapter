package ui

import (
	"bytes"
	"testing"
)

func containsName(list []string, name string) bool {
	for _, n := range list {
		if n == name {
			return true
		}
	}
	return false
}

func requireAssetDirEntries(t *testing.T, path string, expected ...string) {
	t.Helper()

	entries, err := AssetDir(path)
	if err != nil {
		t.Fatalf("AssetDir(%q) returned error: %v", path, err)
	}
	if len(entries) == 0 {
		t.Fatalf("expected AssetDir(%q) to return directory entries", path)
	}
	for _, name := range expected {
		if !containsName(entries, name) {
			t.Fatalf("AssetDir(%q) missing %s", path, name)
		}
	}
}

func requireAssetDirError(t *testing.T, path string) {
	t.Helper()

	if _, err := AssetDir(path); err == nil {
		t.Fatalf("expected AssetDir(%q) to return an error", path)
	}
}

func TestAsset(t *testing.T) {
	data, err := Asset("templates/_base.html")
	if err != nil {
		t.Fatalf("Asset returned error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty asset data")
	}
	if !bytes.Contains(data, []byte("<html")) {
		t.Fatalf("expected asset data to contain HTML content")
	}
}

func TestMustAsset(t *testing.T) {
	data := MustAsset("templates/_base.html")
	if len(data) == 0 {
		t.Fatal("expected non-empty asset data from MustAsset")
	}

	defer func() {
		if recover() == nil {
			t.Fatal("MustAsset did not panic for missing asset")
		}
	}()
	_ = MustAsset("templates/not-found.html")
}

func TestAssetInfo(t *testing.T) {
	info, err := AssetInfo("templates/_base.html")
	if err != nil {
		t.Fatalf("AssetInfo returned error: %v", err)
	}
	if info.Name() != "templates/_base.html" {
		t.Fatalf("expected AssetInfo name to be templates/_base.html, got %q", info.Name())
	}
	if info.Size() <= 0 {
		t.Fatalf("expected AssetInfo size > 0, got %d", info.Size())
	}
}

func TestAssetNames(t *testing.T) {
	names := AssetNames()
	if len(names) == 0 {
		t.Fatal("expected AssetNames to return at least one asset")
	}
	contains := func(name string) bool {
		for _, n := range names {
			if n == name {
				return true
			}
		}
		return false
	}
	if !contains("templates/_base.html") {
		t.Fatal("AssetNames missing templates/_base.html")
	}
	if !contains("static/js/jquery.js") {
		t.Fatal("AssetNames missing static/js/jquery.js")
	}
}

func TestAssetDir(t *testing.T) {
	tests := []struct {
		path     string
		expected []string
	}{
		{path: "", expected: []string{"static", "templates"}},
		{path: "templates", expected: []string{"_base.html", "simulation.html", "status.html"}},
		{path: "static/js", expected: []string{"api.js", "bootstrap.min.js", "jquery.js"}},
	}

	for _, tt := range tests {
		requireAssetDirEntries(t, tt.path, tt.expected...)
	}

	requireAssetDirError(t, "templates/_base.html")
	requireAssetDirError(t, "does/not/exist")
}
