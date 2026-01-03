package main

import (
	"os"

	"github.com/coalaura/plain"
	"github.com/coalaura/scfg"
)

var log = plain.New()

func main() {
	home, err := os.UserHomeDir()
	log.MustFail(err)

	log.Println("Parsing ssh config...")

	config, err := scfg.ParseConfig(home)
	log.MustFail(err)

	log.Println("Parsing known hosts...")

	hosts, err := scfg.ParseKnownHosts(home)
	log.MustFail(err)

	_ = config
	_ = hosts
}
