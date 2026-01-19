package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"filippo.io/age"
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

func (s *Server) Run(config *Config) error {
	if s.client == nil {
		return errors.New("not connected")
	}

	date := time.Now().Format("2006_01_02-15_04")

	ext := ".tar.zst"

	if config.Password != "" {
		ext += ".age"
	}

	path := filepath.Join(s.Target, fmt.Sprintf("%s-%s%s", s.Name, date, ext))

	out, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	defer out.Close()

	session, err := s.client.NewSession()
	if err != nil {
		defer os.Remove(path)

		return err
	}

	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		defer os.Remove(path)

		return err
	}

	session.Stderr = os.Stderr

	cmd := fmt.Sprintf("bash -lc 'tar -C / -cf - %s %s | zstd -T0 -3 -q'", s.exclude, s.include)

	err = session.Start(cmd)
	if err != nil {
		defer os.Remove(path)

		return err
	}

	wr := NewCounter(out)

	stop := wr.Start()
	defer stop()

	var (
		writer io.Writer = wr
		closer io.Closer
	)

	if config.Password != "" {
		recipient, err := age.NewScryptRecipient(config.Password)
		if err != nil {
			defer os.Remove(path)

			return err
		}

		recipient.SetWorkFactor(20)

		aw, err := age.Encrypt(writer, recipient)
		if err != nil {
			defer os.Remove(path)

			return err
		}

		writer = aw
		closer = aw
	}

	_, err = io.Copy(writer, stdout)
	if err != nil {
		if closer != nil {
			closer.Close()
		}

		defer os.Remove(path)

		return err
	}

	if closer != nil {
		err = closer.Close()
		if err != nil {
			return err
		}
	}

	return session.Wait()
}
