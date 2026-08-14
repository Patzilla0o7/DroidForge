package main

import (
	"archive/zip"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	androidVersion = "13"
	apiLevel       = "33"
	toolsURL       = "https://dl.google.com/android/repository/commandlinetools-mac-13114758_latest.zip"
	defaultDevice  = "pixel_5"
	defaultGPU     = "swiftshader"
)

type lab struct{ root, sdk, abi, avd string }

func main() {
	root, err := os.Getwd()
	fatalIf(err)
	l := newLab(root)
	if len(os.Args) < 2 {
		usage()
	}
	switch os.Args[1] {
	case "setup":
		setup(l)
	case "create-avd":
		createAVD(l)
	case "start":
		start(l, os.Args[2:])
	case "stop":
		stop(l)
	case "doctor":
		doctor(l)
	default:
		usage()
	}
}

func newLab(root string) lab {
	abi := "x86_64"
	if runtime.GOARCH == "arm64" {
		abi = "arm64-v8a"
	}
	return lab{root: root, sdk: filepath.Join(root, ".android-sdk-macos"), abi: abi, avd: "aosp" + androidVersion + "-security-" + abi}
}

func usage() {
	fmt.Fprintln(os.Stderr, `DroidForge — macOS Android framework-security lab

Usage:
  droidforge setup                    Install the project-local Android SDK
  droidforge create-avd               Create the managed research AVD
  droidforge start [options]          Start the managed AVD
  droidforge stop                     Stop the running Emulator
  droidforge doctor                   Show environment and component status

start options:
  --image-dir DIR     Use custom images from DIR (system.img is required)
  --gpu MODE          GPU mode (default: swiftshader)
  --wipe-data         Reset AVD user data; removes apps and settings
  --cold-boot         Do not load a Quick Boot snapshot
  --no-window         Run headless

Custom image directories may contain system.img, vendor.img, product.img,
ramdisk.img, kernel-ranchu, kernel, userdata.img, or cache.img.`)
	os.Exit(2)
}

func setup(l lab) {
	need("java")
	fatalIf(os.MkdirAll(filepath.Join(l.sdk, ".downloads"), 0755))
	manager := l.tool("cmdline-tools", "latest", "bin", "sdkmanager")
	if _, err := os.Stat(manager); errors.Is(err, os.ErrNotExist) {
		archive := filepath.Join(l.sdk, ".downloads", "commandlinetools-mac.zip")
		fmt.Println("Downloading Android SDK command-line tools…")
		fatalIf(download(toolsURL, archive))
		fatalIf(extractTools(archive, filepath.Join(l.sdk, "cmdline-tools")))
	}
	fatalIf(run(l, strings.NewReader(strings.Repeat("y\n", 1000)), "sdkmanager", "--licenses"))
	image := fmt.Sprintf("system-images;android-%s;default;%s", apiLevel, l.abi)
	fatalIf(run(l, nil, "sdkmanager", "platform-tools", "emulator", "platforms;android-"+apiLevel, image))
	fmt.Printf("SDK ready: %s\nNext: droidforge create-avd && droidforge start\n", l.sdk)
}

func createAVD(l lab) {
	if hasAVD(l) {
		fmt.Printf("AVD already exists: %s\n", l.avd)
		return
	}
	image := fmt.Sprintf("system-images;android-%s;default;%s", apiLevel, l.abi)
	fatalIf(run(l, strings.NewReader("no\n"), "avdmanager", "create", "avd", "--force", "--name", l.avd, "--package", image, "--device", defaultDevice))
	config := filepath.Join(os.Getenv("HOME"), ".android", "avd", l.avd+".avd", "config.ini")
	f, err := os.OpenFile(config, os.O_APPEND|os.O_WRONLY, 0644)
	fatalIf(err)
	defer f.Close()
	_, err = f.WriteString("\nhw.cpu.ncore=4\nhw.ramSize=4096\ndisk.dataPartition.size=8G\nhw.gpu.enabled=yes\nhw.gpu.mode=auto\nhw.keyboard=yes\nshowDeviceFrame=no\n")
	fatalIf(err)
	fmt.Printf("Created AVD: %s\n", l.avd)
}

func start(l lab, args []string) {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	imageDir := fs.String("image-dir", "", "directory containing custom images")
	gpu := fs.String("gpu", defaultGPU, "emulator GPU mode")
	wipe := fs.Bool("wipe-data", false, "clear AVD data")
	cold := fs.Bool("cold-boot", false, "disable snapshot loading")
	headless := fs.Bool("no-window", false, "run headless")
	fs.Parse(args)
	if !hasAVD(l) {
		fatalIf(fmt.Errorf("AVD %q does not exist; run: droidforge create-avd", l.avd))
	}
	cmdArgs := []string{"-avd", l.avd, "-gpu", *gpu, "-no-boot-anim", "-netdelay", "none", "-netspeed", "full"}
	if *wipe {
		cmdArgs = append(cmdArgs, "-wipe-data")
	}
	if *cold {
		cmdArgs = append(cmdArgs, "-no-snapshot-load")
	}
	if *headless {
		cmdArgs = append(cmdArgs, "-no-window")
	}
	if *imageDir != "" {
		custom, err := customImageArgs(*imageDir)
		fatalIf(err)
		fmt.Fprintf(os.Stderr, "Using custom images from %s; ensure they match AVD ABI %s.\n", *imageDir, l.abi)
		cmdArgs = append(cmdArgs, "-no-snapshot", "-writable-system")
		cmdArgs = append(cmdArgs, custom...)
	}
	cmd := exec.Command(l.tool("emulator", "emulator"), cmdArgs...)
	cmd.Env, cmd.Stdin, cmd.Stdout, cmd.Stderr = l.env(), os.Stdin, os.Stdout, os.Stderr
	fatalIf(cmd.Run())
}

func stop(l lab) { fatalIf(run(l, nil, "adb", "emu", "kill")) }

func customImageArgs(dir string) ([]string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	stat, err := os.Stat(filepath.Join(dir, "system.img"))
	if err != nil || stat.IsDir() {
		return nil, fmt.Errorf("custom image directory requires %s", filepath.Join(dir, "system.img"))
	}
	files := []struct{ name, option string }{
		{"system.img", "-system"}, {"vendor.img", "-vendor"}, {"product.img", "-product"},
		{"ramdisk.img", "-ramdisk"}, {"kernel-ranchu", "-kernel"}, {"kernel", "-kernel"},
		{"userdata.img", "-initdata"}, {"cache.img", "-cache"},
	}
	args, kernelAdded := []string{}, false
	for _, file := range files {
		path := filepath.Join(dir, file.name)
		if _, err := os.Stat(path); err == nil {
			if file.option == "-kernel" && kernelAdded {
				continue
			}
			args = append(args, file.option, path)
			kernelAdded = kernelAdded || file.option == "-kernel"
		}
	}
	return args, nil
}

func doctor(l lab) {
	fmt.Printf("Project: %s\nSDK: %s\nHost ABI: %s\nAVD: %s (%t)\n", l.root, l.sdk, l.abi, l.avd, hasAVD(l))
	for _, item := range []struct{ name, path string }{
		{"sdkmanager", l.tool("cmdline-tools", "latest", "bin", "sdkmanager")},
		{"emulator", l.tool("emulator", "emulator")}, {"adb", l.tool("platform-tools", "adb")},
		{"system image", filepath.Join(l.sdk, "system-images", "android-"+apiLevel, "default", l.abi, "package.xml")},
	} {
		_, err := os.Stat(item.path)
		status := "missing"
		if err == nil {
			status = "ready"
		}
		fmt.Printf("%-14s %s\n", item.name+":", status)
	}
}

func hasAVD(l lab) bool {
	_, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".android", "avd", l.avd+".avd"))
	return err == nil
}
func (l lab) tool(parts ...string) string { return filepath.Join(append([]string{l.sdk}, parts...)...) }
func (l lab) env() []string {
	return append(os.Environ(), "ANDROID_SDK_ROOT="+l.sdk, "ANDROID_HOME="+l.sdk, "PATH="+filepath.Join(l.sdk, "cmdline-tools", "latest", "bin")+":"+filepath.Join(l.sdk, "emulator")+":"+filepath.Join(l.sdk, "platform-tools")+":"+os.Getenv("PATH"))
}
func run(l lab, in io.Reader, name string, args ...string) error {
	paths := map[string][]string{"sdkmanager": {"cmdline-tools", "latest", "bin", "sdkmanager"}, "avdmanager": {"cmdline-tools", "latest", "bin", "avdmanager"}, "adb": {"platform-tools", "adb"}}
	cmd := exec.Command(l.tool(paths[name]...), args...)
	cmd.Env, cmd.Stdin, cmd.Stdout, cmd.Stderr = l.env(), in, os.Stdout, os.Stderr
	return cmd.Run()
}
func need(name string) {
	if _, err := exec.LookPath(name); err != nil {
		fatalIf(fmt.Errorf("required command missing: %s", name))
	}
}
func fatalIf(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func download(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func extractTools(archive, destination string) error {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, entry := range r.File {
		name := filepath.Clean(entry.Name)
		if strings.HasPrefix(name, "..") {
			return fmt.Errorf("unsafe archive path: %s", entry.Name)
		}
		target := filepath.Join(destination, name)
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		src, err := entry.Open()
		if err != nil {
			return err
		}
		dst, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, entry.Mode())
		if err != nil {
			src.Close()
			return err
		}
		_, copyErr := io.Copy(dst, src)
		closeErr := dst.Close()
		src.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return os.Rename(filepath.Join(destination, "cmdline-tools"), filepath.Join(destination, "latest"))
}
