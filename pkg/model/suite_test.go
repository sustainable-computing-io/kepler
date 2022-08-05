package model

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"os/exec"
	"fmt"
	"os"
	"time"
)

func TestModels(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Model Suite")
}


var errCh chan error	
var cmd *exec.Cmd

func StartPyEstimator() {
    cmd = exec.Command("python3", "../../estimator/py/estimator.py")
	go func() {
		errCh <- cmd.Run()
	}()
	select{
		case err := <- errCh:
			fmt.Printf("error: %v\n", err)		
	}
}

func  Destroy() {
	fmt.Println("Destroy")
	if err := cmd.Process.Kill(); err != nil {
		fmt.Printf("failed to kill process: %v\n", err)
	} else {
		fmt.Println("kill estimator")
	}
}


var _ = BeforeSuite(func() {
	errCh = make(chan error)
	go StartPyEstimator()
	for {
		time.Sleep(3*time.Second)
		if _, err := os.Stat(SERVE_SOCKET); err == nil {
			break
		}
	}
})

var _ = AfterSuite(func() {
	defer Destroy()
})
