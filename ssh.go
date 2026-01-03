package main

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/coalaura/scfg"
	"golang.org/x/crypto/ssh"
)

func (s *Server) Connect(home string, config scfg.Config, hosts scfg.KnownHosts) error {
	server, ok := config[s.Name]
	if !ok {
		return fmt.Errorf("unknown ssh server %q", s.Name)
	}

	auth, err := server.AuthMethod(home, nil)
	if err != nil {
		return err
	}

	addr := server.Addr()

	timeout := server.Timeout(10 * time.Second)

	cfg := &ssh.ClientConfig{
		User:            server.DefaultUser(),
		Auth:            auth,
		HostKeyCallback: hosts.HostKeyCallback(),
		Timeout:         timeout,
		Config: ssh.Config{
			Ciphers: []string{
				"aes128-gcm@openssh.com",
				"chacha20-poly1305@openssh.com",
				"aes256-gcm@openssh.com",
				"aes128-ctr",
				"aes256-ctr",
			},
		},
	}

	dialer := net.Dialer{
		Timeout: timeout,
	}

	netConn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return err
	}

	sshConn, channels, requests, err := ssh.NewClientConn(netConn, addr, cfg)
	if err != nil {
		netConn.Close()

		return err
	}

	s.client = ssh.NewClient(sshConn, channels, requests)

	return nil
}

func (s *Server) Close() error {
	if s.client == nil {
		return nil
	}

	return s.client.Close()
}

func (s *Server) Run() error {
	if s.client == nil {
		return errors.New("not connected")
	}

	// TODO: implement

	return nil
}
