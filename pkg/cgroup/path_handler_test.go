/*
Copyright 2021.

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

package cgroup

import (
	"os"
	"reflect"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/sustainable-computing-io/kepler/pkg/utils"
)

func TestReadUInt64(t *testing.T) {
	g := NewWithT(t)

	var testcases = []struct {
		name      string
		contents  string
		expectErr bool
		expectR   uint64
	}{
		{
			name:      "test valid content",
			contents:  "123456",
			expectErr: false,
			expectR:   123456,
		},
		{
			// current we expect to get UINT
			name:      "test invalid content",
			contents:  "dummy",
			expectR:   0,
			expectErr: true,
		},
	}
	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {
			file, err := utils.CreateTempFile(testcase.contents)
			g.Expect(err).NotTo(HaveOccurred())
			defer os.Remove(file)

			d, err := ReadUInt64(file)
			if testcase.expectErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}

			g.Expect(d).To(Equal(testcase.expectR))
		})
	}
}

var testContents1 = `
usage_usec 1
user_usec 2
system_usec 3`

var testContents2 = `
usage_usec 1
dummy dummy   // not acceptable, ignore
user_usec 2
system_usec 3`

var testResults = map[string]interface{}{
	"usage_usec":  uint64(1),
	"user_usec":   uint64(2),
	"system_usec": uint64(3),
}

func TestReadKV(t *testing.T) {
	g := NewWithT(t)

	var testcases = []struct {
		name          string
		contents      string
		expectErr     bool
		fileName      string
		expectResults map[string]interface{}
	}{
		{
			name:          "test valid input",
			contents:      testContents1,
			expectErr:     false,
			fileName:      "",
			expectResults: testResults,
		},
		{
			name:          "test valid input",
			contents:      testContents2,
			expectErr:     false,
			fileName:      "",
			expectResults: testResults,
		},
		{
			name:      "test invalid data",
			contents:  "not valid",
			expectErr: true,
			fileName:  "/tmp/this_file_do_not_exist_dummy",
		},
	}
	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {
			var fileName string
			var err error
			if testcase.fileName == "" {
				fileName, err = utils.CreateTempFile(testcase.contents)
				g.Expect(err).NotTo(HaveOccurred())
				defer os.Remove(fileName)
			} else {
				fileName = testcase.fileName
			}

			d, err := ReadKV(fileName)

			if testcase.expectErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(3).To(Equal(len(d)))
				eq := reflect.DeepEqual(testcase.expectResults, d)
				g.Expect(true).To(Equal(eq))
			}
		})
	}
}
