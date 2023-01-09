package cgroup

import (
	"os"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/sustainable-computing-io/kepler/pkg/utils"
)

const iostatContent1 = `
252:0 rbytes=135168 wbytes=0 rios=2 wios=0 dbytes=0 dios=0`

const iostatContent2 = `
252:0 rbytes=135168 wbytes=0 rios=2 wios=0 dbytes=0 dios=0
16:8 rbytes=1 wbytes=10000 rios=2 wios=0 dbytes=0 dios=0`

func TestReadIOStat(t *testing.T) {
	g := NewWithT(t)

	var testcases = []struct {
		name        string
		contents    string
		expectR     uint64
		expectW     uint64
		expectDisks int
		expectErr   bool
	}{
		{
			name:        "test io status 1",
			contents:    iostatContent1,
			expectR:     135168,
			expectW:     0,
			expectDisks: 1,
		},
		{
			name:        "test io status 2",
			contents:    iostatContent2,
			expectR:     135169,
			expectW:     10000,
			expectDisks: 2,
		},
	}
	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {
			file, err := utils.CreateTempFile(testcase.contents)
			g.Expect(err).NotTo(HaveOccurred())
			defer os.Remove(file)

			r, w, disks, err := readIOStat(file)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(r).To(Equal(testcase.expectR))
			g.Expect(w).To(Equal(testcase.expectW))
			g.Expect(disks).To(Equal(testcase.expectDisks))
		})
	}
}
