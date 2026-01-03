package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"

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

	client *ssh.Client
	args   string
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

	var args bytes.Buffer

	for _, file := range s.Files {
		if len(file) == 0 {
			continue
		}

		if args.Len() > 0 {
			args.WriteByte(' ')
		}

		if file[0] == '!' {
			args.WriteString("--exclude ")

			file = file[1:]
		}

		if file[0] == '/' {
			file = file[1:]
		}

		args.WriteString(file)
	}

	if args.Len() == 0 {
		return errors.New("invalid files")
	}

	s.args = args.String()

	return nil
}
