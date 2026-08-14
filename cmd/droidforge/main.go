package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const (
	defaultAPI = "33"
	toolsURL   = "https://dl.google.com/android/repository/commandlinetools-mac-13114758_latest.zip"
	defaultGPU = "swiftshader"
)

type lab struct{ root, sdk, abi, defaultAVD string }

func main() {
	root, err := os.Getwd()
	fatalIf(err)
	l := newLab(root)
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "setup":
		setup(l, os.Args[2:])
	case "image":
		imageCommand(l, os.Args[2:])
	case "build":
		buildCommand(l, os.Args[2:])
	case "avd":
		avdCommand(l, os.Args[2:])
	case "start":
		start(l, os.Args[2:])
	case "stop":
		stop(l)
	case "doctor":
		doctor(l)
	case "create-avd":
		createAVD(l, []string{}) // Compatibility alias.
	default:
		usage()
	}
}

func newLab(root string) lab {
	abi := "x86_64"
	if runtime.GOARCH == "arm64" {
		abi = "arm64-v8a"
	}
	return lab{root: root, sdk: filepath.Join(root, ".android-sdk-macos"), abi: abi, defaultAVD: "aosp13-security-" + abi}
}

func (l lab) profilesDir() string         { return filepath.Join(l.root, ".droidforge", "profiles") }
func (l lab) tool(parts ...string) string { return filepath.Join(append([]string{l.sdk}, parts...)...) }

func usage() {
	fmt.Fprintln(os.Stderr, `DroidForge — macOS Android framework-security lab

Usage:
  droidforge setup [options]                 Install SDK and one official image profile
  droidforge image install [options]         Download and register an official image
  droidforge image list                      List registered image profiles
  droidforge build import [options]          Register a custom AOSP image directory
  droidforge avd create [options]            Create an AVD from an official profile
  droidforge avd list                        List local AVDs
  droidforge start [options]                 Start an AVD with an optional profile
  droidforge stop                            Stop the running Emulator
  droidforge doctor                          Show environment and profile status

Examples:
  droidforge image install --api 33 --tag default --name aosp13
  droidforge avd create --name research --image aosp13
  droidforge build import --name aosp13-dev --dir artifacts/aosp13-dev
  droidforge start --avd research --image aosp13-dev --cold-boot`)
	os.Exit(2)
}

func fatalIf(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
