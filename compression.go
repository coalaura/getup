package main

import (
	"fmt"
	"strconv"
)

type CompressionAlgorithm struct {
	Extension     string
	CommandPrefix string
	CommandSuffix string
	DefaultLevel  int
	MinimumLevel  int
	MaximumLevel  int
}

var compressionAlgorithms = map[string]CompressionAlgorithm{
	"none": {
		Extension: "",
	},
	"zstd": {
		Extension:     ".zst",
		CommandPrefix: "zstd -T0 -",
		CommandSuffix: " -q",
		DefaultLevel:  3,
		MinimumLevel:  1,
		MaximumLevel:  19,
	},
	"gzip": {
		Extension:     ".gz",
		CommandPrefix: "gzip -",
		CommandSuffix: " -c",
		DefaultLevel:  6,
		MinimumLevel:  1,
		MaximumLevel:  9,
	},
	"xz": {
		Extension:     ".xz",
		CommandPrefix: "xz -",
		CommandSuffix: " -c",
		DefaultLevel:  6,
		MinimumLevel:  0,
		MaximumLevel:  9,
	},
	"lz4": {
		Extension:     ".lz4",
		CommandPrefix: "lz4 -",
		CommandSuffix: " -q -c",
		DefaultLevel:  1,
		MinimumLevel:  1,
		MaximumLevel:  12,
	},
	"bzip2": {
		Extension:     ".bz2",
		CommandPrefix: "bzip2 -",
		CommandSuffix: " -c",
		DefaultLevel:  9,
		MinimumLevel:  1,
		MaximumLevel:  9,
	},
	"zip": {
		Extension:     ".zip",
		CommandPrefix: "zip -q -",
		CommandSuffix: " - -",
		DefaultLevel:  6,
		MinimumLevel:  0,
		MaximumLevel:  9,
	},
}

func (c CompressionAlgorithm) command(level *int) (string, error) {
	if c.CommandPrefix == "" {
		if level != nil {
			return "", fmt.Errorf("compression level is not supported for none")
		}

		return "", nil
	}

	selectedLevel := c.DefaultLevel

	if level != nil {
		selectedLevel = *level
	}

	if selectedLevel < c.MinimumLevel || selectedLevel > c.MaximumLevel {
		return "", fmt.Errorf("compression level must be between %d and %d", c.MinimumLevel, c.MaximumLevel)
	}

	return c.CommandPrefix + strconv.Itoa(selectedLevel) + c.CommandSuffix, nil
}
