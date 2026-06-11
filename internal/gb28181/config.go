package gb28181

import (
	"fmt"
	"net"
)

// Config holds GB28181 SIP signaling configuration.
type Config struct {
	Enabled   bool   `yaml:"enabled"`
	Host      string `yaml:"host"`      // SIP listen host (empty = auto-detect)
	Port      int    `yaml:"port"`      // SIP listen port (default 5060)
	ID        string `yaml:"id"`        // 20-digit platform SIP ID
	Password  string `yaml:"password"`  // Global device registration password
	MediaIP   string `yaml:"media_ip"`  // IP address put in SDP for media reception
	MediaPort int    `yaml:"media_port"` // RTP media port (0=auto/multi-port, >0=single port mode)

	// Upstream platform cascading
	Platforms []PlatformConfig `yaml:"platforms,omitempty"`
}

// GetDomain extracts the first 10 chars of the SIP ID as the GB domain.
func (c *Config) GetDomain() string {
	if len(c.ID) >= 10 {
		return c.ID[:10]
	}
	return c.ID
}

// Validate checks the configuration for correctness.
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	if len(c.ID) < 10 || len(c.ID) > 20 {
		return fmt.Errorf("gb28181.id must be 10-20 digits, got %d", len(c.ID))
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("gb28181.port must be 1-65535, got %d", c.Port)
	}
	if c.MediaIP == "" {
		return fmt.Errorf("gb28181.media_ip is required")
	}
	if net.ParseIP(c.MediaIP) == nil {
		return fmt.Errorf("gb28181.media_ip must be a valid IP address, got %s", c.MediaIP)
	}
	return nil
}

// ApplyDefaults sets default values for the configuration.
func (c *Config) ApplyDefaults() {
	if c.Port == 0 {
		c.Port = 5060
	}
}
