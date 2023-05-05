package ssh

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"strings"
)

func AccessToIP(ip string) error {
	knownHostsFile, err := getKnownHostsFilePath()
	if err != nil {
		return err
	}

	content, err := os.ReadFile(knownHostsFile)
	if err != nil {
		return err
	}

	r := regexp.MustCompile(strings.ReplaceAll(ip, ".", `\.`))
	oldLines := make([]string, 0)
	for _, line := range strings.Split(string(content), "\n") {
		if r.MatchString(line) {
			continue
		}
		oldLines = append(oldLines, line)
	}

	newKnownHostsEntries, err := newEntries(ip)
	if err != nil {
		return err
	}

	lines := append(oldLines, newKnownHostsEntries...)

	return os.WriteFile(knownHostsFile, []byte(strings.Join(lines, "\n")), 0600)
}

func newEntries(ip string) ([]string, error) {
	cmd := exec.Command("ssh-keyscan", ip)
	knownHosts, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	var lines []string
	for _, line := range bytes.Split(knownHosts, []byte{'\n'}) {
		if len(line) == 0 || line[0] == '#' {
			continue
		}
		lines = append(lines, string(line))
	}

	return lines, nil
}

func getKnownHostsFilePath() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/.ssh/known_hosts", u.HomeDir), nil
}
