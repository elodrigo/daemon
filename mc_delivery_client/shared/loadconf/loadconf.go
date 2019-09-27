package loadconf

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
	"regexp"
)

// ConfigBlock : Setting File Block
type ConfigBlock struct {
	DBInfo struct {
		DBUser     string
		DBPassword string
		DBName     string
	}
	IPInfo struct {
		IP   string
		Port string
	}
}

//ConfigInfo : Setting
var ConfigInfo = &ConfigBlock{}

var MCDELIVERYROOTDIR string

func SetConfigController() {
	if MCDELIVERYROOTDIR = os.Getenv("ROOTDIR_MCDELIVERY"); MCDELIVERYROOTDIR == "" {
		log.Println("[ERR]File not found : config.json. Please check your environment variables. [ROOTDIR_MCDELIVERY]")
		// Exit Code 1 : Configuration File Read Error
		os.Exit(1)
	}

	confPath := MCDELIVERYROOTDIR + "/config/config.json"
	var confBuf []byte
	file, err := os.Open(confPath)
	if err != nil {
		log.Println(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if matched, _ := regexp.Match("/\\*.*\\*/", scanner.Bytes()); !matched {
			confBuf = append(confBuf[:], scanner.Bytes()[:]...)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Println(err)
	}
	err = json.Unmarshal(confBuf, ConfigInfo)
	if err != nil {
		log.Println(err)
	}
}
