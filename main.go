package main

import (
	"os"
	"strings"

	"github.com/coalaura/plain"
	"github.com/coalaura/scfg"
)

var Version = "dev"

var log = plain.New()

func main() {
	filters := make(map[string]bool, len(os.Args)-1)

	for _, arg := range os.Args[1:] {
		if arg == "-v" || arg == "--version" {
			log.Printf("getup %s\n", Version)

			return
		}

		filters[strings.ToLower(arg)] = true
	}

	home, err := os.UserHomeDir()
	log.MustFail(err)

	log.Println("Loading config...")

	cfg, err := LoadConfig(home)
	log.MustFail(err)

	log.Println("Parsing ssh config...")

	config, err := scfg.ParseConfig(home)
	log.MustFail(err)

	log.Println("Parsing known hosts...")

	hosts, err := scfg.ParseKnownHosts(home)
	log.MustFail(err)

	for _, task := range cfg.Tasks {
		err := handle(home, task, config, hosts, cfg, filters)
		log.MustFail(err)
	}
}

func handle(home string, task *Task, config scfg.Config, hosts scfg.KnownHosts, cfg *Config, filters map[string]bool) error {
	base := task.ArchiveBase()

	if len(filters) > 0 && !filters[strings.ToLower(base)] {
		return nil
	}

	log.Printf("Connecting to %s...\n", task.Server)

	err := task.Connect(home, config, hosts)
	if err != nil {
		return err
	}

	defer task.Close()

	if len(task.Pre) > 0 {
		log.Printf("Running pre-backup scripts for %s...\n", base)

		err = task.RunPreScripts()
		if err != nil {
			return err
		}
	}

	log.Printf("Backing up %s...\n", base)

	err = task.Run(cfg)
	if err != nil {
		return err
	}

	log.Printf("Completed backing up %s\n", base)

	if len(task.Post) > 0 {
		log.Printf("Running post-backup scripts for %s...\n", base)

		err = task.RunPostScripts()
		if err != nil {
			return err
		}
	}

	return nil
}
