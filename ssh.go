package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"filippo.io/age"
	"github.com/coalaura/scfg"
	"golang.org/x/crypto/ssh"
)

func (t *Task) Connect(home string, config scfg.Config, hosts scfg.KnownHosts) error {
	server, ok := config[t.Server]
	if !ok {
		return fmt.Errorf("unknown ssh server %q", t.Server)
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

	t.client = ssh.NewClient(sshConn, channels, requests)

	return nil
}

func (t *Task) Close() error {
	if t.client == nil {
		return nil
	}

	return t.client.Close()
}

func (t *Task) RunPreScripts() error {
	return t.runCommandsOnRemote(t.Pre)
}

func (t *Task) RunPostScripts() error {
	return t.runCommandsOnRemote(t.Post)
}

func (t *Task) Run(config *Config) error {
	if t.client == nil {
		return errors.New("not connected")
	}

	date := time.Now().Format("2006_01_02-15_04")
	path := filepath.Join(t.Target, fmt.Sprintf("%s-%s%s", t.ArchiveBase(), date, t.archiveExtension(config.Password != "")))

	session, err := t.client.NewSession()
	if err != nil {
		return err
	}

	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return err
	}

	session.Stderr = os.Stderr

	err = session.Start(t.backupCommand())
	if err != nil {
		return err
	}

	return writeBackupFile(path, config.Password, stdout, session.Wait)
}

func (t *Task) archiveExtension(encrypted bool) string {
	extension := ".tar.zst"

	if t.Command != "" {
		extension = ".zst"
	}

	if encrypted {
		extension += ".age"
	}

	return extension
}

func (t *Task) backupCommand() string {
	var source string

	if t.Command != "" {
		source = "(" + t.Command + ")"
	} else {
		source = fmt.Sprintf("tar -C / -cf - %s %s", t.exclude, t.include)
	}

	script := fmt.Sprintf("set -o pipefail; %s | zstd -T0 -3 -q", source)

	return "bash -lc " + shellQuote(script)
}

func (t *Task) runCommandsOnRemote(cmds []string) error {
	for _, cmd := range cmds {
		session, err := t.client.NewSession()
		if err != nil {
			return err
		}

		session.Stdout = os.Stdout
		session.Stderr = os.Stderr

		err = session.Run(cmd)

		session.Close()

		if err != nil {
			return err
		}
	}

	return nil
}

func writeBackupFile(path string, password string, source io.Reader, wait func() error) error {
	partialPath := path + ".partial"

	out, err := os.OpenFile(partialPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	var committed bool

	defer func() {
		if committed {
			return
		}

		out.Close()
		os.Remove(partialPath)
	}()

	wr := NewCounter(out)

	stop := wr.Start()
	defer stop()

	var (
		writer io.Writer = wr
		closer io.Closer
	)

	if password != "" {
		recipient, err := age.NewScryptRecipient(password)
		if err != nil {
			return err
		}

		recipient.SetWorkFactor(20)

		aw, err := age.Encrypt(writer, recipient)
		if err != nil {
			return err
		}

		writer = aw
		closer = aw
	}

	_, err = io.Copy(writer, source)
	if err != nil {
		if closer != nil {
			closer.Close()
		}

		return err
	}

	if closer != nil {
		err = closer.Close()
		if err != nil {
			return err
		}
	}

	err = wait()
	if err != nil {
		return err
	}

	err = out.Close()
	if err != nil {
		return err
	}

	err = os.Rename(partialPath, path)
	if err != nil {
		return err
	}

	committed = true

	return nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
