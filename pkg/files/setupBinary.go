package files

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func SetupBinary(r io.Reader, destinationFolder, binaryName string) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("binary [%s] not found in tarball", binaryName)
		}
		if err != nil {
			return err
		}
		if header.Typeflag == tar.TypeReg && strings.TrimPrefix(header.Name, "./") == binaryName {
			target := filepath.Join(destinationFolder, header.Name)

			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			if _, err := io.Copy(f, tr); err != nil {
				return err
			}

			err = os.Chmod(header.Name, 0o755)
			if err != nil {
				return fmt.Errorf("error setting permissions on file: %v", err)
			}

			f.Close()
			fmt.Println("Downloaded ./" + target)
		}
	}
}
