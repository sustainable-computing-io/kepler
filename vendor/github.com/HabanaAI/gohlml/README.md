# gohlml
The repository provides a set of C wrappers of Habana's `libhlml.so` library (HLML) in Go. The library is meant to be integrated into other modules requiring HLML access.

## Testing
To test the code, transfer the repository to a server where the Habana driver is installed and run the following: 
```shell
go test
```

## Code Cover
To validate metrics code coverage, run: 
```shell
go test -cover
```

# Quick Start
The code below shows an example of using gohlml to query all of the Gaudi(s) on your system and print out their ModuleID.

```go
package main

import (
	"fmt"
	"log"

	hlml "github.com/HabanaAI/gohlml"
)

func main() {
	ret := hlml.Initialize()
	if ret != nil {
		log.Fatalf("failed to initialize HLML: %v", err)
	}
	defer func() {
		ret := hlml.Shutdown()
		if ret != nil {
			log.Fatalf("failed to shutdown HLML: %v", err)
		}
	}()

	count, ret := hlml.DeviceCount()
	if ret != nil {
		log.Fatalf("failed to get device count: %v",err)
	}

	for i := 0; i < count; i++ {
		device, ret := hlml.DeviceHandleByIndex(i)
		if ret != nil {
			log.Fatalf("failed to get device at index %d: %v", i, err)
		}

		uuid, ret := device.ModuleID()
		if ret != nil {
			log.Fatalf("failed to get uuid of device at index %d: %v", i, err)
		}

		fmt.Printf("%v\n", uuid)
	}
}
```