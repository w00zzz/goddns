package config

type Config struct {
	General		GeneralConfig		`json:"general"`
	DNS			DNSConfig			`json:"dns"`
	Discovery	DiscoveryConfig		`json:"discovery"`
	State		StateConfig			`json:"state"`
}

type GeneralConfig struct {
	Interval int					`json:"interval"`
	LogLevel string					`json:"log_level"`
}

type DNSConfig struct {
	Provider	string				`json:"provider"`
	Domain		string				`json:"domain"`
	Host		string				`json:"host"`
	Credentials	map[string]string	`json:"credentials"`
}

type DiscoveryConfig struct {
	IPv4			bool			`json:"ipv4"`
	IPv6			bool			`json:"ipv6"`
	Sources			[]string		`json:"sources"`
	MinConsensus	int				`json:"min_consensus"`
}

type StateConfig struct {
	Path string			`json:"path"`
}
