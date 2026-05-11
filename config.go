package main

import (
	"log"
	"os"
	"strings"

	"gopkg.in/yaml.v1"
)

type Config struct {
	Port            string
	Debug           bool
	Database        string
	AdminPassword   string
	DisableDelete   bool
	BannerImagePath string
	CSRFKey         string
}

// LoadConfig imports the configuration from the first config file found,
// then applies any environment variable overrides on top.
func LoadConfig(confpath string) (config Config) {
	for _, path := range []string{confpath, "config.yaml", "/etc/go-periodical-rack/config.yaml"} {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			config = ParseConfig(path)
			break
		}
	}
	applyEnvOverrides(&config)
	return
}

// ParseConfig does the actual convert into the struct.
func ParseConfig(confpath string) (config Config) {
	file, err := os.ReadFile(confpath)
	if err != nil {
		log.Println("open config: ", confpath, " Error", err)
	}

	if err = yaml.Unmarshal(file, &config); err != nil {
		log.Println("parse config: ", err)

	}
	return
}

// applyEnvOverrides replaces any Config field with its environment variable
// value when the variable is non-empty, allowing container deployments to
// configure the app without a config file on disk.
//
// Variables: TANPORT, TANDEBUG, DATABASE_URL, TANADMINPASS, TANCSRFKEY,
// TANDISABLEDELETE, TANBANNER
func applyEnvOverrides(c *Config) {
	if v := os.Getenv("TANPORT"); v != "" {
		c.Port = v
	}
	if v := os.Getenv("TANDEBUG"); v != "" {
		c.Debug = strings.ToLower(v) == "true"
	}
	if v := os.Getenv("DATABASE_URL"); v != "" {
		c.Database = v
	}
	if v := os.Getenv("TANADMINPASS"); v != "" {
		c.AdminPassword = v
	}
	if v := os.Getenv("TANCSRFKEY"); v != "" {
		c.CSRFKey = v
	}
	if v := os.Getenv("TANDISABLEDELETE"); v != "" {
		c.DisableDelete = strings.ToLower(v) == "true"
	}
	if v := os.Getenv("TANBANNER"); v != "" {
		c.BannerImagePath = v
	}
}
