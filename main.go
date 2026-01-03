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

	log.Println("Loading config...")

	servers, err := LoadConfig(home)
	log.MustFail(err)

	log.Println("Parsing ssh config...")

	config, err := scfg.ParseConfig(home)
	log.MustFail(err)

	log.Println("Parsing known hosts...")

	hosts, err := scfg.ParseKnownHosts(home)
	log.MustFail(err)

	for _, server := range servers.Servers {
		err := handle(home, server, config, hosts)
		log.MustFail(err)
	}
}

func handle(home string, server *Server, config scfg.Config, hosts scfg.KnownHosts) error {
	log.Printf("Connecting to %s...\n", server.Name)

	err := server.Connect(home, config, hosts)
	if err != nil {
		return err
	}

	defer server.Close()

	return nil
}
