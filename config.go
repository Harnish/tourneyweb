package main

import (
	"io/ioutil"
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
	file, err := ioutil.ReadFile(confpath)
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
	if strings.ToLower(tandebug) == "true" {
		config.Debug = true
	} else {
		config.Debug = false
	}
	dbtype := os.Getenv("TANDBTYPE")
	if dbtype == "mysql" {
		dbhost := os.Getenv("TANDBHOST")
		dbuser := os.Getenv("TANDBUSER")
		dbpass := os.Getenv("TANDBPASSWORD")
		dbport := os.Getenv("TANDBPORT")
		dbdb := os.Getenv("TANDBDATABASE")
		config.Database = "mysql://" + dbuser + ":" + dbpass + "@tcp(" + dbhost + ":" + dbport + ")/" + dbdb
	} else {
		config.Database = os.Getenv("TANDB")
	}
	config.AdminPassword = os.Getenv("TANADMINPASS")
	return
}
