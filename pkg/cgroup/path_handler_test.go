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
