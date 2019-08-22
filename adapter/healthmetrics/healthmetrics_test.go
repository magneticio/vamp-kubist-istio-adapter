package healthmetrics_test

import (
	"os"
	"testing"

	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/healthmetrics"
	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/models"
	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/vampclientprovider"
	kubeclient "github.com/magneticio/vampkubistcli/kubernetes"
	"github.com/magneticio/vampkubistcli/logging"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"net/http"
	"net/http/httptest"
)

func createTestServer(fn func(w http.ResponseWriter, r *http.Request)) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(fn))
}

type k8sClientProviderMock struct {
	Host string
}

func (mock k8sClientProviderMock) Get(configPath string) (*kubernetes.Clientset, error) {
	cfg := rest.Config{
		Host: mock.Host,
	}
	return kubernetes.NewForConfig(&cfg)
}

func CreateMockedK8s(t *testing.T, fileName string) *httptest.Server {
	js, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Errorf("Cannot read metrics json file - %v", err)
	}
	ts := createTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("Method: %v", r.Method)
		t.Logf("Path: %v", r.URL.Path)
		switch {
		case r.URL.Path == "/apis/apps/v1/namespaces/logging/deployments":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(js)
		default:
		}
	})

	kubeclient.K8sClient = k8sClientProviderMock{Host: ts.URL}

	return ts
}

func TestProcessHealthMetrics(t *testing.T) {
	logging.Init(os.Stdout, os.Stderr)
	logging.Verbose = true

	ts := CreateMockedK8s(t, "deployments_test.json")
	defer ts.Close()

	vampclientprovider.VirtualCluster = "logging"

	err := healthmetrics.InitK8sClient()
	assert.Nil(t, err)

	// number of deployments in test json
	num := 3

	ch := make(chan *models.LogInstance, num)

	healthmetrics.Process(ch)

	var in [3]*models.LogInstance

	for i := 0; i < num; i++ {
		in[i] = <-ch
		t.Logf("---Got log instance: %v", in[i])
	}

	elastic := in[0]
	assert.Equal(t, "elasticsearch", elastic.DestinationLabels["app"])
	assert.Equal(t, int64(1), elastic.Values["ObservedGeneration"])
	assert.Equal(t, int32(1), elastic.Values["Replicas"])
	assert.Equal(t, int32(1), elastic.Values["UpdatedReplicas"])
	assert.Equal(t, int32(1), elastic.Values["ReadyReplicas"])
	assert.Equal(t, int32(1), elastic.Values["AvailableReplicas"])
	assert.Equal(t, int32(0), elastic.Values["UnavailableReplicas"])
	assert.Equal(t, int32(1), elastic.Values["Availability"])
}
