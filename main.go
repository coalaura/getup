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

	if !shouldRun(task, filters) {
		return nil
	}

	log.Printf("Connecting to %s...\n", task.Server)

	err := task.Connect(home, config, hosts)
	if err != nil {
		return err
	}

	defer task.Close()

	log.Printf("Backing up %s...\n", base)

	if task.IgnoreFailedRead {
		log.Printf("Ignoring failed tar reads for %s...\n", base)
	}

	err = task.Run(cfg)
	if err != nil {
		return err
	}

	log.Printf("Completed backing up %s\n", base)

	return nil
}

func shouldRun(task *Task, filters map[string]bool) bool {
	if len(filters) == 0 {
		return !task.Disabled
	}

	name := strings.ToLower(task.ArchiveBase())
	if filters[name] {
		return true
	}

	for filter, enabled := range filters {
		if enabled && matchesPattern(name, filter) {
			return true
		}
	}

	return false
}

func matchesPattern(name, pattern string) bool {
	prefix, pattern, wildcard := strings.Cut(pattern, "*")
	if !wildcard {
		return name == prefix
	}

	if !strings.HasPrefix(name, prefix) {
		return false
	}

	name = name[len(prefix):]

	for {
		segment, remaining, wildcard := strings.Cut(pattern, "*")
		if !wildcard {
			return strings.HasSuffix(name, segment)
		}

		index := strings.Index(name, segment)
		if index < 0 {
			return false
		}

		name = name[index+len(segment):]
		pattern = remaining
	}
}
