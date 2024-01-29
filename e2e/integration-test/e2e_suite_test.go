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

package integrationtest

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	keplerMetric   *TestKeplerMetric
	kubeconfigPath string
	port           string = "9102"
	ctx            context.Context
	namespace      string = "kepler"
)

func TestE2eTest(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2eTest Suite")
}

var _ = BeforeSuite(func() {
	var err error
	kubeconfigPath = getEnvOrDefault("KUBECONFIG", "")
	keplerMetric, err = NewTestKeplerMetric(kubeconfigPath, namespace, port)
	Expect(err).NotTo(HaveOccurred())

	// for stress test only
	// create stress-ng deployment to generate load
	stressTest := getEnvOrDefault("ENABLE_STRESS_TEST", "")
	if stressTest == "true" {
		err = keplerMetric.createdStressNGDeployment()
		Expect(err).NotTo(HaveOccurred())
	}

	ctx = context.Background()
	err = keplerMetric.RetrievePodNames(ctx)
	Expect(err).NotTo(HaveOccurred())
})
