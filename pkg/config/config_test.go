/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import (
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type mockconfig struct {
}

var mockc mockconfig
var tempfile string

func (c mockconfig) getSysInfo() ([]byte, error) {
	return []byte(""), nil
}

func (c mockconfig) getCgroupV2File() string {
	return tempfile
}

func createTempFile(contents string) (filename string, reterr error) {
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

var _ = Describe("Test Configuration", func() {
	It("Test cgroup version", func() {
		file, err := createTempFile("")
		Expect(err).NotTo(HaveOccurred())
		defer os.Remove(file)
		tempfile = file

		Expect(true).To(Equal(isCGroupV2(mockc)))

		tempfile = "/tmp/this_file_do_not_exist"
		Expect(false).To(Equal(isCGroupV2(mockc)))
	})
})
