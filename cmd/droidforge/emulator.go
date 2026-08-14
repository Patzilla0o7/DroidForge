package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func buildCommand(l lab, args []string) {
	if len(args) == 0 || args[0] != "import" {
		usage()
	}
	fs := flag.NewFlagSet("build import", flag.ExitOnError)
	name := fs.String("name", "", "custom profile name")
	dir := fs.String("dir", "", "directory containing AOSP image files")
	api := fs.String("api", defaultAPI, "Android API level")
	abi := fs.String("abi", l.abi, "build ABI")
	base := fs.String("base", "", "optional official base profile name")
	fs.Parse(args[1:])
	if err := validateName(*name); err != nil {
		fatalIf(err)
	}
	imageDir, err := filepath.Abs(*dir)
	fatalIf(err)
	_, err = customImageArgs(imageDir)
	fatalIf(err)
	if *base != "" {
		p, err := l.loadProfile(*base)
		fatalIf(err)
		if p.Source != "official" {
			fatalIf(fmt.Errorf("base profile must be official"))
		}
	}
	p := profile{Name: *name, Source: "custom", API: *api, ABI: *abi, ImageDir: imageDir, BaseImage: *base}
	fatalIf(l.saveProfile(p))
	fmt.Printf("Imported custom profile %q\n  images: %s\n", p.Name, p.ImageDir)
}

func avdCommand(l lab, args []string) {
	if len(args) == 0 {
		usage()
	}
	switch args[0] {
	case "create":
		createAVD(l, args[1:])
	case "list":
		listAVDs()
	default:
		usage()
	}
}

func createAVD(l lab, args []string) {
	fs := flag.NewFlagSet("avd create", flag.ExitOnError)
	name := fs.String("name", l.defaultAVD, "AVD name")
	image := fs.String("image", "android-"+defaultAPI+"-default-"+l.abi, "official image profile name")
	device := fs.String("device", "pixel_5", "hardware device definition")
	fs.Parse(args)
	if err := validateName(*name); err != nil {
		fatalIf(err)
	}
	p, err := l.loadProfile(*image)
	fatalIf(err)
	if p.Source != "official" {
		fatalIf(fmt.Errorf("AVD creation requires an official image profile; %q is custom", p.Name))
	}
	if hasAVD(*name) {
		fmt.Printf("AVD already exists: %s\n", *name)
		return
	}
	fatalIf(runTool(l, strings.NewReader("no\n"), "avdmanager", "create", "avd", "--force", "--name", *name, "--package", p.Package, "--device", *device))
	config := filepath.Join(os.Getenv("HOME"), ".android", "avd", *name+".avd", "config.ini")
	f, err := os.OpenFile(config, os.O_APPEND|os.O_WRONLY, 0644)
	fatalIf(err)
	defer f.Close()
	_, err = f.WriteString("\nhw.cpu.ncore=4\nhw.ramSize=4096\ndisk.dataPartition.size=8G\nhw.gpu.enabled=yes\nhw.gpu.mode=auto\nhw.keyboard=yes\nshowDeviceFrame=no\n")
	fatalIf(err)
	fmt.Printf("Created AVD %q from profile %q\n", *name, p.Name)
}

func listAVDs() {
	base := filepath.Join(os.Getenv("HOME"), ".android", "avd")
	entries, err := os.ReadDir(base)
	if os.IsNotExist(err) {
		fmt.Println("No AVDs.")
		return
	}
	fatalIf(err)
	for _, entry := range entries {
		if entry.IsDir() && strings.HasSuffix(entry.Name(), ".avd") {
			fmt.Println(strings.TrimSuffix(entry.Name(), ".avd"))
		}
	}
}

func start(l lab, args []string) {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	avd := fs.String("avd", l.defaultAVD, "AVD name")
	image := fs.String("image", "", "registered image profile name")
	imageDir := fs.String("image-dir", "", "custom image directory (legacy shorthand)")
	gpu := fs.String("gpu", defaultGPU, "emulator GPU mode")
	wipe := fs.Bool("wipe-data", false, "clear AVD data")
	cold := fs.Bool("cold-boot", false, "disable snapshot loading")
	headless := fs.Bool("no-window", false, "run headless")
	fs.Parse(args)
	if *image != "" && *imageDir != "" {
		fatalIf(fmt.Errorf("use either --image or --image-dir, not both"))
	}
	if !hasAVD(*avd) {
		fatalIf(fmt.Errorf("AVD %q does not exist; run: droidforge avd create --name %s", *avd, *avd))
	}
	cmdArgs := []string{"-avd", *avd, "-gpu", *gpu, "-no-boot-anim", "-netdelay", "none", "-netspeed", "full"}
	if *wipe {
		cmdArgs = append(cmdArgs, "-wipe-data")
	}
	if *cold {
		cmdArgs = append(cmdArgs, "-no-snapshot-load")
	}
	if *headless {
		cmdArgs = append(cmdArgs, "-no-window")
	}
	if *image != "" {
		p, err := l.loadProfile(*image)
		fatalIf(err)
		if p.Source == "custom" {
			cmdArgs = appendCustomArgs(cmdArgs, p.ImageDir)
		}
	}
	if *imageDir != "" {
		cmdArgs = appendCustomArgs(cmdArgs, *imageDir)
	}
	cmd := exec.Command(l.tool("emulator", "emulator"), cmdArgs...)
	cmd.Env, cmd.Stdin, cmd.Stdout, cmd.Stderr = l.env(), os.Stdin, os.Stdout, os.Stderr
	fatalIf(cmd.Run())
}

func appendCustomArgs(args []string, dir string) []string {
	custom, err := customImageArgs(dir)
	fatalIf(err)
	fmt.Fprintf(os.Stderr, "Using custom images from %s\n", dir)
	return append(args, append([]string{"-no-snapshot", "-writable-system"}, custom...)...)
}

func stop(l lab) { fatalIf(runTool(l, nil, "adb", "emu", "kill")) }

func customImageArgs(dir string) ([]string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	stat, err := os.Stat(filepath.Join(dir, "system.img"))
	if err != nil || stat.IsDir() {
		return nil, fmt.Errorf("custom image directory requires %s", filepath.Join(dir, "system.img"))
	}
	files := []struct{ name, option string }{{"system.img", "-system"}, {"vendor.img", "-vendor"}, {"product.img", "-product"}, {"ramdisk.img", "-ramdisk"}, {"kernel-ranchu", "-kernel"}, {"kernel", "-kernel"}, {"userdata.img", "-initdata"}, {"cache.img", "-cache"}}
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

func hasAVD(name string) bool {
	_, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".android", "avd", name+".avd"))
	return err == nil
}

func doctor(l lab) {
	fmt.Printf("Project: %s\nSDK: %s\nHost ABI: %s\n", l.root, l.sdk, l.abi)
	for _, item := range []struct{ name, path string }{{"sdkmanager", l.tool("cmdline-tools", "latest", "bin", "sdkmanager")}, {"emulator", l.tool("emulator", "emulator")}, {"adb", l.tool("platform-tools", "adb")}} {
		_, err := os.Stat(item.path)
		state := "missing"
		if err == nil {
			state = "ready"
		}
		fmt.Printf("%-14s %s\n", item.name+":", state)
	}
	profiles, err := l.listProfiles()
	fatalIf(err)
	fmt.Printf("Profiles: %d\n", len(profiles))
	for _, p := range profiles {
		fmt.Printf("  %-22s %s\n", p.Name, profileLocation(p))
	}
}

func profileLocation(p profile) string {
	if p.Source == "custom" {
		return p.ImageDir
	}
	return p.Package
}
