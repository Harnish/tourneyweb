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

// LoadConfig imports the configuration.
func LoadConfig(confpath string) (config Config) {
	if confpath != "" {
		_, err := os.Stat(confpath)
		if err == nil {
			config = ParseConfig(confpath)
			return
		}
	}
	confpath = "config.yaml"
	_, err := os.Stat(confpath)
	if err == nil {
		config = ParseConfig(confpath)
		return
	}
	confpath = "/etc/go-periodical-rack/config.yaml"
	_, err = os.Stat(confpath)
	if err == nil {
		config = ParseConfig(confpath)
		return
	}
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

func GetEnvironmentConfig() (config Config) {
	config.Port = os.Getenv("TANPORT")
	tandebug := os.Getenv("TANDEBUG")
	config.Debug = strings.ToLower(tandebug) == "true"
	config.Database = os.Getenv("TANDB")
	config.AdminPassword = os.Getenv("TANADMINPASS")
	return
}
