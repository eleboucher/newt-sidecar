package config

import "flag"

type Config struct {
	GatewayName      string
	GatewayNamespace string
	Namespace        string
	Output           string
	SiteID           string
	TargetHostname   string
	TargetPort       int
	TargetMethod     string
	DenyCountries    string
	SSL              bool
	AnnotationPrefix string
}

func Load() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.GatewayName, "gateway-name", "", "Gateway name to filter HTTPRoutes (required)")
	flag.StringVar(&cfg.GatewayNamespace, "gateway-namespace", "", "Gateway namespace (empty = any)")
	flag.StringVar(&cfg.Namespace, "namespace", "", "Watch namespace (empty = all)")
	flag.StringVar(&cfg.Output, "output", "/etc/newt/blueprint.yaml", "Output blueprint file path")
	flag.StringVar(&cfg.SiteID, "site-id", "", "Pangolin site nice ID (required)")
	flag.StringVar(&cfg.TargetHostname, "target-hostname", "", "Backend gateway hostname (required)")
	flag.IntVar(&cfg.TargetPort, "target-port", 443, "Backend gateway port")
	flag.StringVar(&cfg.TargetMethod, "target-method", "https", "Backend method (http/https/h2c)")
	flag.StringVar(&cfg.DenyCountries, "deny-countries", "", "Comma-separated country codes to deny")
	flag.BoolVar(&cfg.SSL, "ssl", true, "Enable SSL on resources")
	flag.StringVar(&cfg.AnnotationPrefix, "annotation-prefix", "newt-sidecar", "Annotation prefix for per-resource overrides")

	flag.CommandLine.Init("", flag.ExitOnError)
	flag.Parse()

	return cfg
}
