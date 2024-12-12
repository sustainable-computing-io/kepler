/*
Copyright 2024.

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

package kubelet

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	. "github.com/onsi/gomega"
)

func TestDoFetchPod(t *testing.T) {
	g := NewWithT(t)
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "dummy")
	}))
	defer svr.Close()
	res, err := doFetchPod(svr.URL)
	g.Expect(err).NotTo(HaveOccurred())
	defer res.Body.Close()
	g.Expect(res.StatusCode).To(Equal(http.StatusOK))
	bearerToken = "dummy"
	res, err = httpGet("", svr.URL)
	g.Expect(err).NotTo(HaveOccurred())
	defer res.Body.Close()
	g.Expect(res.StatusCode).To(Equal(http.StatusOK))
}

func TestDoFetchPodWithError(t *testing.T) {
	g := NewWithT(t)
	svr := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "dummy")
	}))
	res, err := doFetchPod(svr.URL)
	g.Expect(err).To(HaveOccurred())
	g.Expect(res).To(BeNil())
	if res != nil {
		defer res.Body.Close()
	}
	res, err = httpGet("", svr.URL)
	if res != nil {
		defer res.Body.Close()
	}
	g.Expect(err).To(HaveOccurred())
	g.Expect(res).To(BeNil())
}

func TestLoadToken(t *testing.T) {
	g := NewWithT(t)
	tmpDir, err := os.MkdirTemp("", "kepler-tmp-")
	g.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tmpDir)

	TokenFile, err := os.CreateTemp(tmpDir, "kubeletToken")
	g.Expect(err).NotTo(HaveOccurred())
	_, err = TokenFile.WriteString("token")
	g.Expect(err).NotTo(HaveOccurred())
	TokenFile.Close()

	token, err := loadToken(TokenFile.Name())
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(token).To(Equal("Bearer token"))

	var once sync.Once
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() {
			w.WriteHeader(http.StatusUnauthorized)
		})
		fmt.Fprintf(w, "dummy")
	}))
	defer svr.Close()

	res, err := httpGet(TokenFile.Name(), svr.URL)
	g.Expect(err).NotTo(HaveOccurred())
	defer res.Body.Close()
	g.Expect(res.StatusCode).To(Equal(http.StatusOK))
}

func TestHttpGet(t *testing.T) {
	g := NewWithT(t)
	tmpDir, err := os.MkdirTemp("", "kepler-tmp-")
	g.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tmpDir)

	TokenFile, err := os.CreateTemp(tmpDir, "kubeletToken")
	g.Expect(err).NotTo(HaveOccurred())
	_, err = TokenFile.WriteString("token")
	g.Expect(err).NotTo(HaveOccurred())
	TokenFile.Close()

	bearerToken = ""
	g.Expect(bearerToken).To(Equal("")) // need this to pass lint
	var once sync.Once
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() {
			w.WriteHeader(http.StatusUnauthorized)
		})
		fmt.Fprintf(w, "dummy")
	}))
	defer svr.Close()

	res, err := httpGet(TokenFile.Name(), svr.URL)
	g.Expect(err).NotTo(HaveOccurred())
	defer res.Body.Close()
	g.Expect(res.StatusCode).To(Equal(http.StatusOK))
}
