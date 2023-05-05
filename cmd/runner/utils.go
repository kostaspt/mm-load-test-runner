package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"regexp"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"load-test-runner/internal/cli"
)

func gitSwitchToBranch(ctx context.Context, dir, branch string) error {
	cmd := exec.CommandContext(ctx, "git", "switch", branch)
	cmd.Dir = dir
	return cli.RunCommandWithOutputPrinter(cmd)
}

func runLoadTestCtlCmd(ctx context.Context, args ...string) error {
	as := append([]string{"run", "./cmd/ltctl"}, args...)
	cmd := exec.CommandContext(ctx, "go", as...)
	cmd.Dir = loadTestDir
	log.Println(cmd.String())
	return cli.RunCommandWithOutputPrinter(cmd)
}

func runLoadTestCtlCmdWithOutput(ctx context.Context, args ...string) (string, error) {
	as := append([]string{"run", "./cmd/ltctl"}, args...)
	cmd := exec.CommandContext(ctx, "go", as...)
	cmd.Dir = loadTestDir

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %s", err, string(out))
	}

	return string(out), nil
}

func runLoadTestSSHCmd(ctx context.Context, innerCmd string) error {
	ip, err := getAppIP()
	if err != nil {
		return err
	}

	return runThroughSSH(ip, innerCmd)
}

func runThroughSSH(ip, cmd string) error {
	conn, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		return err
	}

	a := agent.NewClient(conn)

	config := &ssh.ClientConfig{
		User: "ubuntu",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeysCallback(a.Signers),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	sshc, err := ssh.Dial("tcp", ip+":22", config)
	if err != nil {
		return err
	}

	session, err := sshc.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	return session.Run(cmd)
}

func getAppIP() (string, error) {
	info, err := runLoadTestCtlCmdWithOutput(context.Background(), "deployment", "info")
	if err != nil {
		return "", err
	}

	r := regexp.MustCompile(clusterName + `-app-0:\s+(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})`)
	m := r.FindStringSubmatch(info)
	if len(m) == 0 {
		return "", fmt.Errorf("could not find app IP in output: %s", info)
	}

	return m[1], nil
}

func getDBHost() (string, error) {
	info, err := runLoadTestCtlCmdWithOutput(context.Background(), "deployment", "info")
	if err != nil {
		return "", err
	}

	// until end of line or new line
	r := regexp.MustCompile(`DB writer endpoint:\s+(.+)[\n$]`)
	m := r.FindStringSubmatch(info)
	if len(m) == 0 {
		return "", fmt.Errorf("could not find DB writer endpoint in output: %s", info)
	}

	return m[1], nil
}

func updateLoadTestTarball(path string) error {
	config, err := os.ReadFile(loadTestDir + "/config/deployer.json")
	if err != nil {
		return err
	}

	var d map[string]interface{}
	if err = json.Unmarshal(config, &d); err != nil {
		return err
	}

	d["LoadTestDownloadURL"] = "file://" + loadTestDir + "/" + path

	config, err = json.MarshalIndent(d, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(loadTestDir+"/config/deployer.json", config, 0644)
}

func waitForLoadTestToFinish() {
	ctx, cancel := context.WithTimeout(context.Background(), durationTotal)
	defer cancel()

	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Second):
			elapsedTime := time.Since(startTime)
			timeLeft := durationTotal - elapsedTime
			fmt.Printf("Time left: %s\n", timeLeft.Round(time.Second))
		}
	}
}

func generateReport(ctx context.Context, name string, startTime time.Time, endTime time.Time) error {
	layout := "2006-01-02 15:04:05"
	return runLoadTestCtlCmd(ctx,
		"report", "generate",
		"--output", name+"-"+runHash+".out",
		"--label", name,
		startTime.UTC().Format(layout),
		endTime.UTC().Format(layout),
	)
}
