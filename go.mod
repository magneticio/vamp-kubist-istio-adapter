module github.com/magneticio/vamp-kubist-istio-adapter

go 1.12

replace (
	k8s.io/api => k8s.io/api v0.0.0-20190409021203-6e4e0e4f393b
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190404173353-6a84e37a896d
	k8s.io/client-go => k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
)

require (
	github.com/gogo/protobuf v1.3.1
	github.com/imdario/mergo v0.3.8 // indirect
	github.com/json-iterator/go v1.1.7 // indirect
	github.com/magneticio/vampkubistcli v0.0.57
	github.com/mhausenblas/kubecuddler v0.0.0-20181012110128-5836f3e4e7d0
	github.com/montanaflynn/stats v0.5.0
	github.com/mxschmitt/golang-combinations v1.0.0
	github.com/spf13/viper v1.4.0
	github.com/stretchr/testify v1.3.0
	golang.org/x/crypto v0.0.0-20191029031824-8986dd9e96cf // indirect
	google.golang.org/genproto v0.0.0-20191028173616-919d9bdd9fe6 // indirect
	google.golang.org/grpc v1.23.0
	gopkg.in/yaml.v2 v2.2.4
	istio.io/api v0.0.0-20190820204432-483f2547d882
	istio.io/gogo-genproto v0.0.0-20191009201739-17d570f95998 // indirect
	istio.io/istio v0.0.0-20191011003621-3cfabd9b36bc
	k8s.io/api v0.0.0-20190918195907-bd6ac527cfd2
	k8s.io/apimachinery v0.0.0-20190817020851-f2f3a405f61d
	k8s.io/client-go v10.0.0+incompatible
	k8s.io/klog v0.4.0 // indirect
)
