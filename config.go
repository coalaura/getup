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
	Servers []*Server `yaml:"servers"`
}

type Server struct {
	Name   string   `yaml:"name"`
	Target string   `yaml:"target"`
	Files  []string `yaml:"files"`

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
	for _, server := range c.Servers {
		err := server.Parse()
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Server) Parse() error {
	if s.Name == "" {
		return errors.New("missing server name")
	}

	if s.Target == "" {
		return errors.New("missing target directory")
	}

	if len(s.Files) == 0 {
		return errors.New("missing files")
	}

	var (
		include strings.Builder
		exclude strings.Builder
	)

	for _, file := range s.Files {
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

	s.include = include.String()
	s.exclude = exclude.String()

	return nil
}
