package cli

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
)

func RunCommandWithOutputPrinter(cmd *exec.Cmd) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	err = cmd.Start()
	if err != nil {
		return err
	}

	stdoutReader := bufio.NewReader(stdout)
	stderrReader := bufio.NewReader(stderr)

	go func() {
		for {
			line, _, err1 := stdoutReader.ReadLine()
			if err1 == io.EOF {
				break
			}
			if err1 != nil {
				fmt.Println("Error reading stdout:", err1)
				break
			}
			fmt.Println(string(line))
		}
	}()

	go func() {
		for {
			line, _, err1 := stderrReader.ReadLine()
			if err1 == io.EOF {
				break
			}
			if err1 != nil {
				fmt.Println("Error reading stderr:", err1)
				break
			}
			fmt.Println(string(line))
		}
	}()

	err = cmd.Wait()
	if err != nil {
		return err
	}

	return nil
}
