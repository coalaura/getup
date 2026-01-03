package main

import "golang.org/x/crypto/ssh"

type SSH struct {
	conn *ssh.Client
}

func NewSSH(host, port string) (*SSH, error) {
	cfg := &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Config: ssh.Config{
			// Prefer AEAD ciphers first (usually fastest / lowest overhead).
			Ciphers: []string{
				"aes128-gcm@openssh.com",
				"chacha20-poly1305@openssh.com",
				"aes256-gcm@openssh.com",
				// Fallbacks if server doesn’t offer GCM/ChaCha:
				"aes128-ctr",
				"aes256-ctr",
			},

			// Optional: reduce rekey frequency for large streams (trade-off: security).
			// RekeyThreshold: 1 << 40, // ~1 TB
		},
	}

	_ = cfg

	return nil, nil
}
