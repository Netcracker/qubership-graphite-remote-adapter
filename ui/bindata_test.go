package ui

import (
	"bytes"
	"testing"
)

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
		if r := recover(); r == nil {
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
	dirs, err := AssetDir("")
	if err != nil {
		t.Fatalf("AssetDir(\"\") returned error: %v", err)
	}
	if len(dirs) == 0 {
		t.Fatal("expected AssetDir(\"\") to return directory entries")
	}
	contains := func(list []string, name string) bool {
		for _, n := range list {
			if n == name {
				return true
			}
		}
		return false
	}
	if !contains(dirs, "static") {
		t.Fatal("AssetDir(\"\") missing static directory")
	}
	if !contains(dirs, "templates") {
		t.Fatal("AssetDir(\"\") missing templates directory")
	}

	files, err := AssetDir("templates")
	if err != nil {
		t.Fatalf("AssetDir(templates) returned error: %v", err)
	}
	if !contains(files, "_base.html") {
		t.Fatal("AssetDir(templates) missing _base.html")
	}
	if !contains(files, "simulation.html") {
		t.Fatal("AssetDir(templates) missing simulation.html")
	}
	if !contains(files, "status.html") {
		t.Fatal("AssetDir(templates) missing status.html")
	}

	jsFiles, err := AssetDir("static/js")
	if err != nil {
		t.Fatalf("AssetDir(static/js) returned error: %v", err)
	}
	if !contains(jsFiles, "api.js") {
		t.Fatal("AssetDir(static/js) missing api.js")
	}
	if !contains(jsFiles, "bootstrap.min.js") {
		t.Fatal("AssetDir(static/js) missing bootstrap.min.js")
	}
	if !contains(jsFiles, "jquery.js") {
		t.Fatal("AssetDir(static/js) missing jquery.js")
	}

	if _, err := AssetDir("templates/_base.html"); err == nil {
		t.Fatal("expected AssetDir on a file path to return an error")
	}
	if _, err := AssetDir("does/not/exist"); err == nil {
		t.Fatal("expected AssetDir on a missing path to return an error")
	}
}
