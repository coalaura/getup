package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"golang.org/x/crypto/ssh"
)

type Config struct {
	Password string  `yaml:"password"`
	Tasks    []*Task `yaml:"tasks"`
}

type Task struct {
	Server  string   `yaml:"server"`
	Name    string   `yaml:"name"`
	Target  string   `yaml:"target"`
	Files   []string `yaml:"files"`
	Command string   `yaml:"command"`
	Pre     []string `yaml:"pre"`
	Post    []string `yaml:"post"`

	client  *ssh.Client
	exclude string
	include string
}

func LoadConfig(home string) (*Config, error) {
	path := filepath.Join(home, ".config", "getup.yml")

	file, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}

	defer file.Close()

	var config Config

	err = yaml.NewDecoder(file).Decode(&config)
	if err != nil {
		return nil, err
	}

	err = config.Parse()
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func (c *Config) Parse() error {
	for _, task := range c.Tasks {
		err := task.Parse()
		if err != nil {
			return err
		}
	}

	return nil
}

func (t *Task) Parse() error {
	if t.Server == "" {
		return errors.New("missing server name")
	}

	if t.Target == "" {
		return errors.New("missing target directory")
	}

	t.Command = strings.TrimSpace(t.Command)

	if len(t.Files) == 0 && t.Command == "" {
		return errors.New("missing files or command")
	}

	if len(t.Files) > 0 && t.Command != "" {
		return errors.New("files and command are mutually exclusive")
	}

	if t.Command != "" {
		return nil
	}

	var (
		include strings.Builder
		exclude strings.Builder
	)

	for _, file := range t.Files {
		if len(file) == 0 {
			continue
		}

		var excl bool

		if file[0] == '!' {
			excl = true

			file = file[1:]
		}

		if file[0] == '/' {
			file = file[1:]
		}

		if excl {
			if exclude.Len() > 0 {
				exclude.WriteByte(' ')
			}

			exclude.WriteString("--exclude ")
			exclude.WriteString(file)
		} else {
			if include.Len() > 0 {
				include.WriteByte(' ')
			}

			include.WriteString(file)
		}
	}

	if include.Len() == 0 {
		return errors.New("invalid files")
	}

	t.include = include.String()
	t.exclude = exclude.String()

	return nil
}

func (t *Task) ArchiveBase() string {
	if t.Name != "" {
		return t.Name
	}

	return t.Server
}
