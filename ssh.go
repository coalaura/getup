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
		User:              server.DefaultUser(),
		Auth:              auth,
		HostKeyCallback:   hosts.HostKeyCallback(),
		HostKeyAlgorithms: hosts.HostKeyAlgorithms(addr),
		Timeout:           timeout,
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

func (t *Task) Run(config *Config) error {
	if t.client == nil {
		return errors.New("not connected")
	}

	date := time.Now().Format("2006_01_02-15_04")
	path := filepath.Join(t.Target, fmt.Sprintf("%s-%s%s", t.ArchiveBase(), date, t.archiveExtension(config.Password != "")))
	partial := path + ".partial"

	out, err := os.OpenFile(partial, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	writeStarted := false

	defer func() {
		if writeStarted {
			return
		}

		out.Close()
		os.Remove(partial)
	}()

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
	session.Stdin = strings.NewReader(t.backupCommand())

	err = session.Start("bash -l -s 3>&1 1>&2")
	if err != nil {
		return err
	}

	writeStarted = true

	return writeBackupFile(path, config.Password, out, stdout, session.Wait)
}

func (t *Task) archiveExtension(encrypted bool) string {
	extension := ".tar" + t.compressor.Extension

	if t.Command != "" {
		extension = t.compressor.Extension
	}

	if encrypted {
		extension += ".age"
	}

	return extension
}

func (t *Task) backupCommand() string {
	var script strings.Builder

	script.WriteString("set -o pipefail\n")
	script.WriteByte('\n')

	writeRemoteCommands(&script, "run_pre", t.Pre, false)
	writeRemoteCommands(&script, "run_post", t.Post, true)

	script.WriteString("cleanup_done=0\n")
	script.WriteString("cleanup() {\n")
	script.WriteString("\tlocal primary_status=$?\n")
	script.WriteString("\tlocal cleanup_status\n\n")
	script.WriteString("\ttrap - EXIT\n")
	script.WriteString("\ttrap '' HUP INT TERM\n\n")
	script.WriteString("\tif (( cleanup_done != 0 )); then\n")
	script.WriteString("\t\texit \"$primary_status\"\n")
	script.WriteString("\tfi\n\n")
	script.WriteString("\tcleanup_done=1\n")
	script.WriteString("\trun_post\n")
	script.WriteString("\tcleanup_status=$?\n\n")
	script.WriteString("\tif (( cleanup_status != 0 )); then\n")
	script.WriteString("\t\tif (( primary_status != 0 )); then\n")
	script.WriteString("\t\t\tprintf 'getup: cleanup failed with status %d; preserving primary status %d\\n' \"$cleanup_status\" \"$primary_status\"\n")
	script.WriteString("\t\telse\n")
	script.WriteString("\t\t\tprimary_status=$cleanup_status\n")
	script.WriteString("\t\tfi\n")
	script.WriteString("\tfi\n\n")
	script.WriteString("\texit \"$primary_status\"\n")
	script.WriteString("}\n\n")

	if t.Command == "" {
		writePrerequisiteCheck(&script, "tar")
	}

	if t.compressor.Executable != "" {
		writePrerequisiteCheck(&script, t.compressor.Executable)
	}

	script.WriteString("trap cleanup EXIT\n")
	script.WriteString("trap 'exit 129' HUP\n")
	script.WriteString("trap 'exit 130' INT\n")
	script.WriteString("trap 'exit 143' TERM\n\n")
	script.WriteString("run_pre\n")
	script.WriteString("primary_status=$?\n")
	script.WriteString("if (( primary_status != 0 )); then\n")
	script.WriteString("\tprintf 'getup: pre command failed with status %d\\n' \"$primary_status\"\n")
	script.WriteString("\texit \"$primary_status\"\n")
	script.WriteString("fi\n\n")

	writeBackupPipeline(&script, t)

	script.WriteString("primary_status=$?\n")
	script.WriteString("if (( primary_status != 0 )); then\n")
	script.WriteString("\tprintf 'getup: backup pipeline failed with status %d\\n' \"$primary_status\"\n")
	script.WriteString("fi\n")
	script.WriteString("exit \"$primary_status\"\n")

	return script.String()
}

func writeBackupFile(path string, password string, out *os.File, source io.Reader, wait func() error) error {
	partialPath := path + ".partial"

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
		writer   io.Writer = wr
		closer   io.Closer
		setupErr error
	)

	if password != "" {
		recipient, err := age.NewScryptRecipient(password)
		if err != nil {
			setupErr = err
		} else {
			recipient.SetWorkFactor(20)

			aw, err := age.Encrypt(writer, recipient)
			if err != nil {
				setupErr = err
			} else {
				writer = aw
				closer = aw
			}
		}
	}

	if setupErr != nil {
		io.Copy(io.Discard, source)

		wait()

		return setupErr
	}

	_, copyErr := io.Copy(writer, source)

	if copyErr != nil {
		io.Copy(io.Discard, source)

	}

	var closeErr error

	if closer != nil {
		closeErr = closer.Close()
	}

	waitErr := wait()

	if copyErr != nil {
		return copyErr
	}

	if closeErr != nil {
		return closeErr
	}

	if waitErr != nil {
		return waitErr
	}

	err := out.Close()
	if err != nil {
		return err
	}

	err = os.Rename(partialPath, path)
	if err != nil {
		return err
	}

	committed = true

	bytes, duration := wr.Stats()

	log.Printf("Backup completed in %s (~%s)\n", fmtDuration(duration), humanSpeed(bytes, duration))

	return nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func writeRemoteCommands(script *strings.Builder, name string, commands []string, continueOnError bool) {
	script.WriteString(name)
	script.WriteString("() {\n")
	script.WriteString("\tlocal command_status\n")

	if continueOnError {
		script.WriteString("\tlocal result=0\n")
	}

	script.WriteByte('\n')

	for _, command := range commands {
		script.WriteString("\t(\n")
		script.WriteString(command)
		script.WriteString("\n\t) 3>&-\n")

		if continueOnError {
			script.WriteString("\tcommand_status=$?\n")
			script.WriteString("\tif (( command_status != 0 )); then\n")
			script.WriteString("\t\tprintf 'getup: post command failed with status %d\\n' \"$command_status\"\n")
			script.WriteString("\t\tif (( result == 0 )); then\n")
			script.WriteString("\t\t\tresult=$command_status\n")
			script.WriteString("\t\tfi\n")
			script.WriteString("\tfi\n\n")
		} else {
			script.WriteString("\tcommand_status=$?\n")
			script.WriteString("\tif (( command_status != 0 )); then\n")
			script.WriteString("\t\treturn \"$command_status\"\n")
			script.WriteString("\tfi\n\n")
		}
	}

	if continueOnError {
		script.WriteString("\treturn \"$result\"\n")
	} else {
		script.WriteString("\treturn 0\n")
	}

	script.WriteString("}\n\n")
}

func writePrerequisiteCheck(script *strings.Builder, executable string) {
	quotedExecutable := shellQuote(executable)

	script.WriteString("if ! command -v -- ")
	script.WriteString(quotedExecutable)
	script.WriteString(" >/dev/null 2>&1; then\n")
	script.WriteString("\tprintf 'getup: required remote executable not found: %s\\n' ")
	script.WriteString(quotedExecutable)
	script.WriteString("\n\texit 127\n")
	script.WriteString("fi\n\n")
}

func writeBackupPipeline(script *strings.Builder, task *Task) {
	if task.Command != "" {
		script.WriteString("(\n")
		script.WriteString(task.Command)
		script.WriteString("\n)")
	} else {
		script.WriteString("tar -C / -cf -")

		for _, exclude := range task.excludes {
			script.WriteString(" --exclude=")
			script.WriteString(shellQuote(exclude))
		}

		script.WriteString(" --")

		for _, include := range task.includes {
			script.WriteByte(' ')
			script.WriteString(shellQuote(include))
		}
	}

	if task.compressorCommand != "" {
		script.WriteString(" | ")
		script.WriteString(task.compressorCommand)
	}

	script.WriteString(" >&3\n")
}
