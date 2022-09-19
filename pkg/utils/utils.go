package utils

import (
	"os"
)

func CreateTempFile(contents string) (filename string, reterr error) {
	f, err := os.CreateTemp("", "")
	if err != nil {
		return "", err
	}
	defer func() {
		if err := f.Close(); err != nil {
			return
		}
	}()
	_, _ = f.WriteString(contents)
	return f.Name(), nil
}
