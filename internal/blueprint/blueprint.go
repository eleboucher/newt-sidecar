package blueprint

import (
	"strings"

	"github.com/home-operations/newt-sidecar/internal/config"
)

type Blueprint struct {
	PublicResources map[string]Resource `yaml:"public-resources"`
}

type Resource struct {
	Name          string   `yaml:"name"`
	Protocol      string   `yaml:"protocol"`
	SSL           bool     `yaml:"ssl"`
	FullDomain    string   `yaml:"full-domain"`
	TLSServerName string   `yaml:"tls-server-name"`
	Rules         []Rule   `yaml:"rules,omitempty"`
	Targets       []Target `yaml:"targets"`
}

type Rule struct {
	Action string `yaml:"action"`
	Match  string `yaml:"match"`
	Value  string `yaml:"value"`
}

type Target struct {
	Site     string `yaml:"site"`
	Hostname string `yaml:"hostname"`
	Method   string `yaml:"method"`
	Port     int    `yaml:"port"`
}

// HostnameToKey converts a hostname to a resource map key.
// Example: "home.erwanleboucher.dev" → "home-erwanleboucher-dev"
func HostnameToKey(hostname string) string {
	return strings.ReplaceAll(hostname, ".", "-")
}

// BuildResource creates a Resource from an HTTPRoute hostname, name, annotations, and config.
func BuildResource(routeName, hostname string, annotations map[string]string, cfg *config.Config) Resource {
	name := routeName
	ssl := cfg.SSL
	prefix := cfg.AnnotationPrefix

	if v, ok := annotations[prefix+"/name"]; ok && v != "" {
		name = v
	}
	if v, ok := annotations[prefix+"/ssl"]; ok {
		ssl = v == "true" || v == "1"
	}

	var rules []Rule
	if cfg.DenyCountries != "" {
		for _, country := range strings.Split(cfg.DenyCountries, ",") {
			country = strings.TrimSpace(country)
			if country != "" {
				rules = append(rules, Rule{
					Action: "deny",
					Match:  "country",
					Value:  country,
				})
			}
		}
	}

	return Resource{
		Name:          name,
		Protocol:      "http",
		SSL:           ssl,
		FullDomain:    hostname,
		TLSServerName: hostname,
		Rules:         rules,
		Targets: []Target{
			{
				Site:     cfg.SiteID,
				Hostname: cfg.TargetHostname,
				Method:   cfg.TargetMethod,
				Port:     cfg.TargetPort,
			},
		},
	}
}
