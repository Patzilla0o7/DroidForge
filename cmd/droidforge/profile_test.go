package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOfficialProfileDefaultsAndPersistence(t *testing.T) {
	temp := t.TempDir()
	l := newLab(temp)
	p, err := officialProfile("33", "default", "x86_64", "")
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "android-33-default-x86_64" {
		t.Fatalf("unexpected name: %s", p.Name)
	}
	if err := l.saveProfile(p); err != nil {
		t.Fatal(err)
	}
	loaded, err := l.loadProfile(p.Name)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Package != "system-images;android-33;default;x86_64" {
		t.Fatalf("unexpected package: %s", loaded.Package)
	}
}

func TestCustomImageArgs(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"system.img", "vendor.img", "kernel-ranchu"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	args, err := customImageArgs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 6 {
		t.Fatalf("got %d args, want 6: %v", len(args), args)
	}
}

func TestCommandLineToolsURL(t *testing.T) {
	for _, hostOS := range []string{"darwin", "linux"} {
		url, err := commandLineToolsURL(hostOS)
		if err != nil || url == "" {
			t.Fatalf("%s: url=%q err=%v", hostOS, url, err)
		}
	}
	if _, err := commandLineToolsURL("windows"); err == nil {
		t.Fatal("expected unsupported OS error")
	}
}
