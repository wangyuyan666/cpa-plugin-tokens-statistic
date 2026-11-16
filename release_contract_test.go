package main

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestReleasePluginIDContract(t *testing.T) {
	const pluginID = "tokens-statistic-fix"
	var registry struct {
		Plugins []struct {
			ID         string `json:"id"`
			Version    string `json:"version"`
			Repository string `json:"repository"`
			Install    struct {
				Type      string `json:"type"`
				Artifacts []struct {
					GOOS   string `json:"goos"`
					GOARCH string `json:"goarch"`
					URL    string `json:"url"`
					SHA256 string `json:"sha256"`
					Size   int64  `json:"size"`
				} `json:"artifacts"`
			} `json:"install"`
		} `json:"plugins"`
	}
	raw, err := os.ReadFile("registry.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &registry); err != nil {
		t.Fatal(err)
	}
	if len(registry.Plugins) != 1 {
		t.Fatalf("registry plugin count = %d, want 1", len(registry.Plugins))
	}
	plugin := registry.Plugins[0]
	if plugin.ID != pluginID {
		t.Fatalf("registry plugin ID = %q, want %q", plugin.ID, pluginID)
	}
	if plugin.Install.Type != "direct" || len(plugin.Install.Artifacts) != 6 {
		t.Fatalf("expected six direct-install artifacts, got %+v", plugin.Install)
	}
	platforms := make(map[string]bool)
	for _, artifact := range plugin.Install.Artifacts {
		platform := artifact.GOOS + "_" + artifact.GOARCH
		wantURL := plugin.Repository + "/releases/download/v" + plugin.Version + "/" + pluginID + "_" + plugin.Version + "_" + platform + ".zip"
		if artifact.URL != wantURL {
			t.Errorf("%s artifact URL = %q, want %q", platform, artifact.URL, wantURL)
		}
		digest, err := hex.DecodeString(artifact.SHA256)
		if err != nil || len(digest) != 32 || artifact.Size <= 0 {
			t.Errorf("%s artifact requires a SHA-256 digest and positive size", platform)
		}
		if platforms[platform] {
			t.Errorf("duplicate platform %q", platform)
		}
		platforms[platform] = true
	}
	for _, platform := range []string{"darwin_amd64", "darwin_arm64", "linux_amd64", "linux_arm64", "windows_amd64", "windows_arm64"} {
		if !platforms[platform] {
			t.Errorf("missing platform %q", platform)
		}
	}

	for _, check := range []struct {
		path string
		want string
	}{
		{".github/workflows/release.yml", "  PLUGIN_ID: " + pluginID + "\n"},
		{"scripts/build-linux-amd64.sh", "PLUGIN_ID=${PLUGIN_ID:-" + pluginID + "}"},
		{"scripts/build-linux-arm64.sh", "PLUGIN_ID=${PLUGIN_ID:-" + pluginID + "}"},
		{"scripts/build_dll.ps1", `$pluginID = if ($env:PLUGIN_ID) { $env:PLUGIN_ID } else { "` + pluginID + `" }`},
		{"README.md", "    " + pluginID + ":\n"},
		{"README_EN.md", "    " + pluginID + ":\n"},
	} {
		data, err := os.ReadFile(check.path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), check.want) {
			t.Errorf("%s must use the fork plugin ID: missing %q", check.path, check.want)
		}
	}
}
