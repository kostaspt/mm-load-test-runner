package main

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"time"

	"load-test-runner/internal/cli"
	"load-test-runner/internal/ssh"
)

func destroyDeployment(ctx context.Context) error {
	return runLoadTestCtlCmd(ctx, "deployment", "destroy")
}

func checkoutServerBranch(branch string) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		return gitSwitchToBranch(ctx, serverDir, branch)
	}
}

func checkoutLoadTestBranch(branch string) func(context.Context) error {
	return func(ctx context.Context) error {
		return gitSwitchToBranch(ctx, loadTestDir, branch)
	}
}

func buildServer(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "make", "build-linux-amd64")
	cmd.Dir = serverDir
	return cli.RunCommandWithOutputPrinter(cmd)
}

func buildLoadTest(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "make", "package")
	cmd.Dir = loadTestDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, string(out))
	}

	r := regexp.MustCompile(`-czf\s+(dist/.+\.tar\.gz)`)
	m := r.FindStringSubmatch(string(out))
	if len(m) == 0 {
		return fmt.Errorf("could not find tarball in output: %s", string(out))
	}

	return updateLoadTestTarball(m[1])
}

func createDeployment(ctx context.Context) error {
	return runLoadTestCtlCmd(ctx, "deployment", "create")
}

func initSSHAccess(ctx context.Context) error {
	ip, err := getAppIP()
	if err != nil {
		return err
	}

	return ssh.AccessToIP(ip)
}

func emptyDatabase(ctx context.Context) error {
	query := "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"
	return runQueryInDB(ctx, query)
}

func setupDatabase(ctx context.Context) error {
	if err := runLoadTestSSHCmd(ctx, exec.Command("/opt/mattermost/bin/mattermost", "db", "reset", "--confirm").String()); err != nil {
		return err
	}

	host, err := getDBHost()
	if err != nil {
		return err
	}

	importTestDB := fmt.Sprintf("export PGPASSWORD=mostest80098bigpass_; curl https://lt-public-data.s3.amazonaws.com/12M_610_psql.sql.gz | zcat | psql -h %s -U mmuser %sdb", host, clusterName)
	if err = runLoadTestSSHCmd(ctx, importTestDB); err != nil {
		return err
	}

	// Temp hack
	// This is needed currently because of issues with the dump file
	if err = runQueryInDB(ctx, "DELETE FROM systems;"); err != nil {
		return err
	}

	return nil
}

func runQueryInDB(ctx context.Context, query string) error {
	host, err := getDBHost()
	if err != nil {
		return err
	}

	cmd := fmt.Sprintf(`export PGPASSWORD=mostest80098bigpass_; psql -h %s -U mmuser %sdb -c %s`, host, clusterName, strconv.Quote(query))
	return runLoadTestSSHCmd(ctx, cmd)
}

func forceRestartService(ctx context.Context) error {
	return runLoadTestSSHCmd(ctx, `sudo systemctl start mattermost`)
}

func resetLoadTest(ctx context.Context) error {
	return runLoadTestCtlCmd(ctx, "loadtest", "reset", "--confirm")
}

func runLoadTest(testName string) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		if err := runLoadTestCtlCmd(ctx, "loadtest", "start"); err != nil {
			return err
		}

		startTime := time.Now()

		waitForLoadTestToFinish()

		if err := stopLoadTest(ctx); err != nil {
			return err
		}

		return generateReport(ctx, testName, startTime, time.Now())
	}
}

func stopLoadTest(ctx context.Context) error {
	_, err := runLoadTestCtlCmdWithOutput(ctx, "loadtest", "stop")
	if err != nil {
		r := regexp.MustCompile(`load-test coordinator with id .+ not found`)
		if r.MatchString(err.Error()) {
			log.Println("Load test already stopped")
			return nil
		}
		return err
	}
	return nil
}

func compareResults(ctx context.Context) error {
	return runLoadTestCtlCmd(ctx,
		"report", "compare",
		"base-"+runHash+".out",
		"new-"+runHash+".out",
		"--output", "results-"+runHash+".txt",
		"--graph",
	)
}
