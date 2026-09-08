package main

import (
	"errors"
	"fmt"
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
	Server           string   `yaml:"server"`
	Name             string   `yaml:"name"`
	Disabled         bool     `yaml:"disabled"`
	Target           string   `yaml:"target"`
	Files            []string `yaml:"files"`
	Command          string   `yaml:"command"`
	Pre              []string `yaml:"pre"`
	Post             []string `yaml:"post"`
	Compression      string   `yaml:"compression"`
	CompressionLevel *int     `yaml:"compression-level"`

	client            *ssh.Client
	compressor        CompressionAlgorithm
	compressorCommand string
	excludes          []string
	includes          []string
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

	err := validateArchiveBase(t.ArchiveBase())
	if err != nil {
		return err
	}

	t.Compression = strings.ToLower(strings.TrimSpace(t.Compression))
	if t.Compression == "" {
		t.Compression = "zstd"
	}

	compressor, ok := compressionAlgorithms[t.Compression]
	if !ok {
		return errors.New("unknown compression algorithm")
	}

	compressorCommand, err := compressor.command(t.CompressionLevel)
	if err != nil {
		return err
	}

	t.compressor = compressor
	t.compressorCommand = compressorCommand

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

	t.includes = t.includes[:0]
	t.excludes = t.excludes[:0]

	for _, file := range t.Files {
		if file == "" {
			return errors.New("invalid empty file path")
		}

		excluded := false

		if file[0] == '!' {
			excluded = true
			file = file[1:]
		}

		if file == "" {
			return errors.New("invalid empty exclusion path")
		}

		if strings.IndexByte(file, 0) >= 0 {
			return errors.New("file path contains NUL")
		}

		if file[0] == '/' {
			file = file[1:]
		}

		if file == "" {
			return errors.New("invalid empty file path")
		}

		if excluded {
			t.excludes = append(t.excludes, file)
		} else {
			t.includes = append(t.includes, file)
		}
	}

	if len(t.includes) == 0 {
		return errors.New("invalid files")
	}

	return nil
}

func (t *Task) ArchiveBase() string {
	if t.Name != "" {
		return t.Name
	}

	return t.Server
}

func validateArchiveBase(base string) error {
	if base == "" || base == "." || base == ".." {
		return errors.New("archive name must be a non-empty filename component")
	}

	if strings.ContainsAny(base, "/\\<>:\"|?*") || filepath.IsAbs(base) || hasControlCharacter(base) {
		return fmt.Errorf("archive name %q must be a single filename component", base)
	}

	return nil
}

func hasControlCharacter(value string) bool {
	for index := range len(value) {
		character := value[index]

		if character < ' ' || character == 0x7f {
			return true
		}
	}

	return false
}
