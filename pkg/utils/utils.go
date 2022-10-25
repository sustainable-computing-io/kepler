package utils

import (
	"encoding/binary"
	"os"
	"unsafe"
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

func DetermineHostByteOrder() binary.ByteOrder {
	var i int32 = 0x01020304
	u := unsafe.Pointer(&i)
	pb := (*byte)(u)
	b := *pb
	if b == 0x04 {
		return binary.LittleEndian
	}

	return binary.BigEndian
}
