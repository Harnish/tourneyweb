package main

import (
	"log"
	"os"
	"strings"

	"gopkg.in/yaml.v1"
)

type Config struct {
	Port                     string
	Debug                    bool
	Database                 string
	DisableDelete            bool
	BannerImagePath          string
	CSRFKey                  string
	SMTPHost                 string
	SMTPPort                 string
	SMTPUsername             string
	SMTPPassword             string
	FromEmail                string
	BaseURL                  string
	DisableEmailVerification bool
}

func LoadConfig(confpath string) (config Config) {
	for _, path := range []string{confpath, "config.yaml", "/config/config.yaml"} {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			config = ParseConfig(path)
			break
		}
	}
	applyEnvOverrides(&config)
	if config.Port == "" {
		config.Port = "8989"
	}
	if config.SMTPPort == "" {
		config.SMTPPort = "587"
	}
	return
}

func ParseConfig(confpath string) (config Config) {
	file, err := os.ReadFile(confpath)
	if err != nil {
		log.Println("open config:", confpath, "Error", err)
	}
	if err = yaml.Unmarshal(file, &config); err != nil {
		log.Println("parse config:", err)
	}
	return
}

// applyEnvOverrides replaces any Config field with its environment variable
// value when the variable is non-empty.
//
// Variables: PORT, DEBUG, DATABASE_URL, CSRF_KEY, DISABLE_DELETE, BANNER_IMAGE_PATH,
// SMTP_HOST, SMTP_PORT, SMTP_USERNAME, SMTP_PASSWORD, FROM_EMAIL, BASE_URL,
// DISABLE_EMAIL_VERIFICATION
func applyEnvOverrides(c *Config) {
	if v := os.Getenv("PORT"); v != "" {
		c.Port = v
	}
	if v := os.Getenv("DEBUG"); v != "" {
		c.Debug = strings.ToLower(v) == "true"
	}
	if v := os.Getenv("DATABASE_URL"); v != "" {
		c.Database = v
	}
	if v := os.Getenv("CSRF_KEY"); v != "" {
		c.CSRFKey = v
	}
	if v := os.Getenv("DISABLE_DELETE"); v != "" {
		c.DisableDelete = strings.ToLower(v) == "true"
	}
	if v := os.Getenv("BANNER_IMAGE_PATH"); v != "" {
		c.BannerImagePath = v
	}
	if v := os.Getenv("SMTP_HOST"); v != "" {
		c.SMTPHost = v
	}
	if v := os.Getenv("SMTP_PORT"); v != "" {
		c.SMTPPort = v
	}
	if v := os.Getenv("SMTP_USERNAME"); v != "" {
		c.SMTPUsername = v
	}
	if v := os.Getenv("SMTP_PASSWORD"); v != "" {
		c.SMTPPassword = v
	}
	if v := os.Getenv("FROM_EMAIL"); v != "" {
		c.FromEmail = v
	}
	if v := os.Getenv("BASE_URL"); v != "" {
		c.BaseURL = v
	}
	if v := os.Getenv("DISABLE_EMAIL_VERIFICATION"); v != "" {
		c.DisableEmailVerification = strings.ToLower(v) == "true"
	}
}
