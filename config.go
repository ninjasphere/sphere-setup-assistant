package main

import (
	"code.google.com/p/gcfg"
	"os/exec"
)

type AssistantConfig struct {
	Wireless_Host struct {
		SSID string
		Key string
		Full_Network_Access bool
	}
}

func LoadConfig(path string) AssistantConfig {
	var cfg AssistantConfig

	// defaults
	uniqueSuffix, _ := exec.Command("/opt/ninjablocks/bin/sphere-go-serial | sha256sum | cut -c1-8").Output()
	cfg.Wireless_Host.SSID = "NinjaSphere-" + string(uniqueSuffix)
	cfg.Wireless_Host.Key = "ninjasphere"
	cfg.Wireless_Host.Full_Network_Access = false
	
	// load from config file (optionally)
	gcfg.ReadFileInto(&cfg, path)
	
	return cfg
}

