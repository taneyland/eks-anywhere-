package conformance

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"time"

	"golang.org/x/sys/unix"

	"github.com/aws/eks-anywhere/pkg/files"
)

const (
	destinationFile = "sonobuoy"
	sonobuoyDarwin  = "https://github.com/vmware-tanzu/sonobuoy/releases/download/v0.53.2/sonobuoy_0.53.2_darwin_amd64.tar.gz"
	sonobuoyLinux   = "https://github.com/vmware-tanzu/sonobuoy/releases/download/v0.53.2/sonobuoy_0.53.2_linux_amd64.tar.gz"
)

func Download() error {
	var err error

	if _, err := os.Stat(destinationFile); err == nil {
		fmt.Println("Nothing downloaded file already exists: " + destinationFile)
		return nil
	}

	var utsname unix.Utsname
	err = unix.Uname(&utsname)
	if err != nil {
		return fmt.Errorf("uname call failure: %v", err)
	}

	var downloadFile string
	sysname := string(bytes.Trim(utsname.Sysname[:], "\x00"))
	if sysname == "Darwin" {
		downloadFile = sonobuoyDarwin
	} else {
		downloadFile = sonobuoyLinux
	}
	fmt.Println("Downloading sonobuoy for " + sysname + ": " + downloadFile)

	var resp *http.Response
	client := &http.Client{
		Timeout: time.Second * 240,
	}
	resp, err = client.Get(downloadFile)
	if err != nil {
		return fmt.Errorf("error opening download: %v", err)
	}
	defer resp.Body.Close()

	err = files.SetupBinary(resp.Body, "", destinationFile)
	if err != nil {
		return err
	}

	return fmt.Errorf("did not find sonobuoy file in download")
}
