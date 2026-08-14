package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func setup(l lab, args []string) {
	if len(args) != 0 {
		fatalIf(fmt.Errorf("setup does not accept image options; use: droidforge image install --api … --tag … --abi … --name …"))
	}
	fatalIf(ensureTools(l))
	fatalIf(runTool(l, strings.NewReader(strings.Repeat("y\n", 1000)), "sdkmanager", "--licenses"))
	fatalIf(runTool(l, nil, "sdkmanager", "platform-tools", "emulator"))
	fmt.Printf("Base SDK environment ready: %s\nNext: droidforge image install --api %s --tag default --name <profile>\n", l.sdk, defaultAPI)
}

func imageCommand(l lab, args []string) {
	if len(args) == 0 {
		usage()
	}
	switch args[0] {
	case "install":
		fs := flag.NewFlagSet("image install", flag.ExitOnError)
		api := fs.String("api", defaultAPI, "Android API level")
		tag := fs.String("tag", "default", "system image tag")
		abi := fs.String("abi", l.abi, "system image ABI")
		name := fs.String("name", "", "saved image profile name")
		fs.Parse(args[1:])
		fatalIf(installOfficial(l, *api, *tag, *abi, *name))
	case "list":
		profiles, err := l.listProfiles()
		fatalIf(err)
		if len(profiles) == 0 {
			fmt.Println("No image profiles. Run: droidforge image install …")
			return
		}
		for _, p := range profiles {
			fmt.Printf("%-24s %-8s api=%-4s abi=%-10s %s\n", p.Name, p.Source, p.API, p.ABI, profileLocation(p))
		}
	default:
		usage()
	}
}

func installOfficial(l lab, api, tag, abi, name string) error {
	p, err := officialProfile(api, tag, abi, name)
	if err != nil {
		return err
	}
	if err := ensureTools(l); err != nil {
		return err
	}
	if err := runTool(l, strings.NewReader(strings.Repeat("y\n", 1000)), "sdkmanager", "--licenses"); err != nil {
		return err
	}
	if err := runTool(l, nil, "sdkmanager", "platform-tools", "emulator", "platforms;android-"+api, p.Package); err != nil {
		return err
	}
	if err := l.saveProfile(p); err != nil {
		return err
	}
	fmt.Printf("Installed official profile %q\n  package: %s\n", p.Name, p.Package)
	return nil
}

func ensureTools(l lab) error {
	if _, err := exec.LookPath("java"); err != nil {
		return fmt.Errorf("JDK 17+ is required: java not found")
	}
	manager := l.tool("cmdline-tools", "latest", "bin", "sdkmanager")
	if _, err := os.Stat(manager); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Join(l.sdk, ".downloads"), 0755); err != nil {
		return err
	}
	archive := filepath.Join(l.sdk, ".downloads", "commandlinetools-mac.zip")
	fmt.Println("Downloading Android SDK command-line tools…")
	if err := download(toolsURL, archive); err != nil {
		return err
	}
	return extractTools(archive, filepath.Join(l.sdk, "cmdline-tools"))
}

func runTool(l lab, in io.Reader, name string, args ...string) error {
	paths := map[string][]string{
		"sdkmanager": {"cmdline-tools", "latest", "bin", "sdkmanager"},
		"avdmanager": {"cmdline-tools", "latest", "bin", "avdmanager"},
		"adb":        {"platform-tools", "adb"},
	}
	path, ok := paths[name]
	if !ok {
		return fmt.Errorf("unknown SDK tool %q", name)
	}
	cmd := exec.Command(l.tool(path...), args...)
	cmd.Env, cmd.Stdin, cmd.Stdout, cmd.Stderr = l.env(), in, os.Stdout, os.Stderr
	return cmd.Run()
}

func (l lab) env() []string {
	return append(os.Environ(), "ANDROID_SDK_ROOT="+l.sdk, "ANDROID_HOME="+l.sdk, "PATH="+filepath.Join(l.sdk, "cmdline-tools", "latest", "bin")+":"+filepath.Join(l.sdk, "emulator")+":"+filepath.Join(l.sdk, "platform-tools")+":"+os.Getenv("PATH"))
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
	latest := filepath.Join(destination, "latest")
	if err := os.RemoveAll(latest); err != nil {
		return err
	}
	return os.Rename(filepath.Join(destination, "cmdline-tools"), latest)
}
