package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type profile struct {
	Name      string `json:"name"`
	Source    string `json:"source"` // official or custom
	API       string `json:"api"`
	Tag       string `json:"tag,omitempty"`
	ABI       string `json:"abi"`
	Package   string `json:"package,omitempty"`
	ImageDir  string `json:"image_dir,omitempty"`
	BaseImage string `json:"base_image,omitempty"`
}

func officialProfile(api, tag, abi, name string) (profile, error) {
	if name == "" {
		name = "android-" + api + "-" + tag + "-" + abi
	}
	if err := validateName(name); err != nil {
		return profile{}, err
	}
	if api == "" || tag == "" || abi == "" {
		return profile{}, fmt.Errorf("api, tag, and abi are required")
	}
	return profile{Name: name, Source: "official", API: api, Tag: tag, ABI: abi, Package: fmt.Sprintf("system-images;android-%s;%s;%s", api, tag, abi)}, nil
}

func validateName(name string) error {
	if name == "" || name == "." || strings.Contains(name, "..") || strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("invalid profile name %q", name)
	}
	return nil
}

func (l lab) saveProfile(p profile) error {
	if err := validateName(p.Name); err != nil {
		return err
	}
	if err := os.MkdirAll(l.profilesDir(), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(l.profilesDir(), p.Name+".json"), append(data, '\n'), 0644)
}

func (l lab) loadProfile(name string) (profile, error) {
	if err := validateName(name); err != nil {
		return profile{}, err
	}
	data, err := os.ReadFile(filepath.Join(l.profilesDir(), name+".json"))
	if err != nil {
		return profile{}, fmt.Errorf("image profile %q: %w", name, err)
	}
	var p profile
	if err := json.Unmarshal(data, &p); err != nil {
		return profile{}, err
	}
	if p.Name != name || (p.Source != "official" && p.Source != "custom") {
		return profile{}, fmt.Errorf("invalid profile %q", name)
	}
	return p, nil
}

func (l lab) listProfiles() ([]profile, error) {
	entries, err := os.ReadDir(l.profilesDir())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	profiles := make([]profile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		p, err := l.loadProfile(name)
		if err != nil {
			return nil, err
		}
		profiles = append(profiles, p)
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].Name < profiles[j].Name })
	return profiles, nil
}
