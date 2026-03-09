package blueprint_test

import (
	"testing"

	"github.com/home-operations/newt-sidecar/internal/blueprint"
	"github.com/home-operations/newt-sidecar/internal/config"
)

func TestHostnameToKey(t *testing.T) {
	tests := []struct {
		hostname string
		want     string
	}{
		{"home.erwanleboucher.dev", "home-erwanleboucher-dev"},
		{"wsflux.erwanleboucher.dev", "wsflux-erwanleboucher-dev"},
		{"simple", "simple"},
	}

	for _, tt := range tests {
		t.Run(tt.hostname, func(t *testing.T) {
			got := blueprint.HostnameToKey(tt.hostname)
			if got != tt.want {
				t.Errorf("HostnameToKey(%q) = %q, want %q", tt.hostname, got, tt.want)
			}
		})
	}
}

func TestBuildResource(t *testing.T) {
	cfg := &config.Config{
		SiteID:           "glistening-desert-rosy-boa",
		TargetHostname:   "kgateway-external.network.svc.cluster.local",
		TargetPort:       443,
		TargetMethod:     "https",
		DenyCountries:    "RU,CN",
		SSL:              true,
		AnnotationPrefix: "newt-sidecar",
	}

	r := blueprint.BuildResource("home-assistant", "home.erwanleboucher.dev", nil, cfg)

	if r.Name != "home-assistant" {
		t.Errorf("Name = %q, want %q", r.Name, "home-assistant")
	}
	if r.Protocol != "http" {
		t.Errorf("Protocol = %q, want %q", r.Protocol, "http")
	}
	if !r.SSL {
		t.Error("SSL should be true")
	}
	if r.FullDomain != "home.erwanleboucher.dev" {
		t.Errorf("FullDomain = %q, want %q", r.FullDomain, "home.erwanleboucher.dev")
	}
	if r.TLSServerName != "home.erwanleboucher.dev" {
		t.Errorf("TLSServerName = %q, want %q", r.TLSServerName, "home.erwanleboucher.dev")
	}
	if len(r.Rules) != 2 {
		t.Errorf("len(Rules) = %d, want 2", len(r.Rules))
	}
	if r.Rules[0].Action != "deny" || r.Rules[0].Match != "country" || r.Rules[0].Value != "RU" {
		t.Errorf("Rules[0] = %+v, want {deny country RU}", r.Rules[0])
	}
	if len(r.Targets) != 1 {
		t.Errorf("len(Targets) = %d, want 1", len(r.Targets))
	}
	if r.Targets[0].Site != "glistening-desert-rosy-boa" {
		t.Errorf("Targets[0].Site = %q, want %q", r.Targets[0].Site, "glistening-desert-rosy-boa")
	}
	if r.Targets[0].Hostname != "kgateway-external.network.svc.cluster.local" {
		t.Errorf("Targets[0].Hostname = %q, want %q", r.Targets[0].Hostname, "kgateway-external.network.svc.cluster.local")
	}
	if r.Targets[0].Port != 443 {
		t.Errorf("Targets[0].Port = %d, want 443", r.Targets[0].Port)
	}
}

func TestBuildResource_AnnotationOverrides(t *testing.T) {
	cfg := &config.Config{
		SiteID:           "test-site",
		TargetHostname:   "gw.local",
		TargetPort:       443,
		TargetMethod:     "https",
		SSL:              true,
		AnnotationPrefix: "newt-sidecar",
	}

	annotations := map[string]string{
		"newt-sidecar/name": "Custom Name",
		"newt-sidecar/ssl":  "false",
	}

	r := blueprint.BuildResource("original-name", "test.example.com", annotations, cfg)

	if r.Name != "Custom Name" {
		t.Errorf("Name = %q, want %q", r.Name, "Custom Name")
	}
	if r.SSL {
		t.Error("SSL should be false after annotation override")
	}
}

func TestBuildResource_NoDenyCountries(t *testing.T) {
	cfg := &config.Config{
		SiteID:           "test-site",
		TargetHostname:   "gw.local",
		TargetPort:       80,
		TargetMethod:     "http",
		SSL:              false,
		AnnotationPrefix: "newt-sidecar",
	}

	r := blueprint.BuildResource("myroute", "myapp.example.com", nil, cfg)

	if len(r.Rules) != 0 {
		t.Errorf("Rules should be empty, got %d rules", len(r.Rules))
	}
}
