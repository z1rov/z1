package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/z1rov/z1/internal/ui"
	"gopkg.in/yaml.v3"
)

const (
	ImageName     = "ghcr.io/z1rov/z1-images/z1-images:latest"
	ContainerName = "z1"

	VersionURL = "https://raw.githubusercontent.com/z1rov/z1-images/refs/heads/main/version/version.txt"

	VersionRegex = `^([\d]+\.[\d]+)`
)

type NetworkConfig struct {
	Mode      string `yaml:"mode"`
	Name      string `yaml:"name"`
	VPNConfig string `yaml:"vpn_config"`
}

type Config struct {
	Shell    string            `yaml:"shell"`
	Network  NetworkConfig     `yaml:"network"`
	Mounts   []string          `yaml:"mounts"`
	Env      map[string]string `yaml:"env"`
	AnvilDir string            `yaml:"anvil_dir"`
}

func defaults() *Config {
	return &Config{
		Shell: "zsh",
		Network: NetworkConfig{
			Mode: "host",
		},
		Mounts: []string{},
		Env:    map[string]string{},
	}
}

func ConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "/root/.config/z1"
	}
	return filepath.Join(home, ".config", "z1")
}

func ConfigPath() string {
	return filepath.Join(ConfigDir(), "config.yml")
}

func Exists() bool {
	_, err := os.Stat(ConfigPath())
	return err == nil
}

func Load() *Config {
	cfg := defaults()

	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		return cfg
	}

	var fileCfg Config
	if err := yaml.Unmarshal(data, &fileCfg); err != nil {
		return cfg
	}

	if fileCfg.Shell != "" {
		cfg.Shell = fileCfg.Shell
	}
	if fileCfg.Network.Mode != "" {
		cfg.Network.Mode = fileCfg.Network.Mode
	}
	if fileCfg.Network.Name != "" {
		cfg.Network.Name = fileCfg.Network.Name
	}
	if fileCfg.Network.VPNConfig != "" {
		cfg.Network.VPNConfig = fileCfg.Network.VPNConfig
	}
	if len(fileCfg.Mounts) > 0 {
		cfg.Mounts = fileCfg.Mounts
	}
	if len(fileCfg.Env) > 0 {
		cfg.Env = fileCfg.Env
	}
	if fileCfg.AnvilDir != "" {
		cfg.AnvilDir = fileCfg.AnvilDir
	}

	return cfg
}

func AnvilDir() string {
	cfg := Load()
	if cfg.AnvilDir != "" {
		return cfg.AnvilDir
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "/anvil"
	}
	return filepath.Join(home, "anvil")
}

const template = `# z1 configuration file
# generated automatically on first "z1 install"

# shell used inside the container (attach + exec)
shell: zsh

network:
  # host | bridge | vpn
  mode: host
  # custom docker network name (bridge/vpn modes only)
  name: ""
  # path to a WireGuard config to auto-connect on start (vpn mode only)
  vpn_config: ""

# extra bind mounts, format: host:container:mode
mounts: []
#  - /home/user/wordlists:/wordlists:ro

# extra environment variables injected into the container
env: {}
#  HTTP_PROXY: "http://127.0.0.1:8080"

# override for the persistent workspace directory (default: ~/anvil)
anvil_dir: ""
`

func WriteTemplate() error {
	if Exists() {
		return nil
	}
	if err := os.MkdirAll(ConfigDir(), 0755); err != nil {
		return fmt.Errorf("mkdir config dir: %w", err)
	}
	return os.WriteFile(ConfigPath(), []byte(template), 0644)
}


func ShowEffective() {
	cfg := Load()

	ui.KV("shell", cfg.Shell, ui.ClrInfo)
	ui.KV("network.mode", cfg.Network.Mode, ui.ClrInfo)
	if cfg.Network.Name != "" {
		ui.KV("network.name", cfg.Network.Name, ui.ClrInfo)
	}
	if cfg.Network.VPNConfig != "" {
		ui.KV("network.vpn_config", cfg.Network.VPNConfig, ui.ClrInfo)
	}
	ui.KV("anvil_dir", AnvilDir(), ui.ClrInfo)

	if len(cfg.Mounts) == 0 {
		ui.KV("mounts", "(none)", ui.ClrDimStr)
	} else {
		for i, m := range cfg.Mounts {
			ui.KV(fmt.Sprintf("mounts[%d]", i), m, ui.ClrInfo)
		}
	}

	if len(cfg.Env) == 0 {
		ui.KV("env", "(none)", ui.ClrDimStr)
	} else {
		for k, v := range cfg.Env {
			ui.KV("env."+k, v, ui.ClrInfo)
		}
	}

	ui.KV("config file", ConfigPath(), ui.ClrDimStr)
	if !Exists() {
		ui.KV("status", "using defaults, no config.yml found", ui.ClrWarn)
	}
}
