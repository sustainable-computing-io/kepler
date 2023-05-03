package co2

import (
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCarbonIntensity(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CO2 Suite")
}

var _ = Describe("CO2 Test Configuration", func() {
	var co2_response = `{ 
		"data":[{ 
			"from": "2023-05-03T19:30Z",
			"to": "2023-05-03T20:00Z",
			"intensity": {
				"forecast": 162,
				"actual": 162,
				"index": "moderate"
			}
		}]
	}`
	var (
		srv     *httptest.Server
		handler http.Handler
	)
	BeforeEach(func() {
		// config.EnabledCO2 = true
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(co2_response))
		})
	})
	JustBeforeEach(func() {
		srv = httptest.NewServer(handler)
	})
	AfterEach(func() {
		srv.CloseClientConnections()
		srv.Close()
	})
	It("Checks the resource path", func() {
		r, _ := co2Impl.GetCarbonIntensity(srv.URL)
		Expect(r).To(Equal(int64(162)))
	})

})
