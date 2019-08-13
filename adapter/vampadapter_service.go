// Copyright © 2018 Developer developer@vamp.io
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package vampadapter

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"os"

	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/config"
	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/models"
	"github.com/magneticio/vamp-kubist-istio-adapter/adapter/processor"
	"istio.io/api/mixer/adapter/model/v1beta1"
	policy "istio.io/api/policy/v1beta1"
	"istio.io/istio/mixer/template/logentry"
)

type (
	// Server is basic server interface
	Server interface {
		Addr() string
		Close() error
		Run(shutdown chan error)
	}

	// VampAdapter supports logentry template.
	VampAdapter struct {
		listener net.Listener
		server   *grpc.Server
	}
)

var _ logentry.HandleLogEntryServiceServer = &VampAdapter{}

/*
type InstanceMsg struct {
	// Name of the instance as specified in configuration.
	Name string `protobuf:"bytes,72295727,opt,name=name,proto3" json:"name,omitempty"`
	// Variables that are delivered for each log entry.
	Variables map[string]*v1beta1.Value `protobuf:"bytes,1,rep,name=variables,proto3" json:"variables,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	// Timestamp is the time value for the log entry
	Timestamp *v1beta1.TimeStamp `protobuf:"bytes,2,opt,name=timestamp,proto3" json:"timestamp,omitempty"`
	// Severity indicates the importance of the log entry.
	Severity string `protobuf:"bytes,3,opt,name=severity,proto3" json:"severity,omitempty"`
	// Optional. An expression to compute the type of the monitored resource this log entry is being recorded on.
	// If the logging backend supports monitored resources, these fields are used to populate that resource.
	// Otherwise these fields will be ignored by the adapter.
	MonitoredResourceType string `protobuf:"bytes,4,opt,name=monitored_resource_type,json=monitoredResourceType,proto3" json:"monitored_resource_type,omitempty"`
	// Optional. A set of expressions that will form the dimensions of the monitored resource this log entry is being
	// recorded on. If the logging backend supports monitored resources, these fields are used to populate that resource.
	// Otherwise these fields will be ignored by the adapter.
	MonitoredResourceDimensions map[string]*v1beta1.Value `protobuf:"bytes,5,rep,name=monitored_resource_dimensions,json=monitoredResourceDimensions,proto3" json:"monitored_resource_dimensions,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
}
*/

// HandleLogEntry records log entries
func (s *VampAdapter) HandleLogEntry(ctx context.Context, r *logentry.HandleLogEntryRequest) (*v1beta1.ReportResult, error) {

	// fmt.Printf("received request %v\n", *r)
	// var b bytes.Buffer
	cfg := &config.Params{}

	if r.AdapterConfig != nil {
		if err := cfg.Unmarshal(r.AdapterConfig.Value); err != nil {
			fmt.Printf("error unmarshalling adapter config: %v", err)
			return nil, err
		}
	}

	s.instances(r.Instances)

	return nil, nil
}

func decodeDimensions(in map[string]*policy.Value) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = decodeValue(v.GetValue())
	}
	return out
}

func decodeValue(in interface{}) interface{} {
	switch t := in.(type) {
	case *policy.Value_StringValue:
		return t.StringValue
	case *policy.Value_Int64Value:
		return t.Int64Value
	case *policy.Value_DoubleValue:
		return t.DoubleValue
	case *policy.Value_IpAddressValue:
		ipV := t.IpAddressValue.Value
		ipAddress := net.IP(ipV)
		str := ipAddress.String()
		return str
	case *policy.Value_DurationValue:
		return t.DurationValue.Value.String()
	case *policy.Value_StringMapValue:
		return t.StringMapValue.Value
	default:
		return fmt.Sprintf("%v", in)
	}
}

// instances conver InstanceMsg to internal LogInstance type and send to processor queue
func (s *VampAdapter) instances(in []*logentry.InstanceMsg) error {

	for _, inst := range in {
		logInstance := &models.LogInstance{
			Timestamp:         inst.Timestamp.Value.Seconds,
			DestinationLabels: make(map[string]string, 0),
			Values:            make(map[string]interface{}, 1),
		}
		// severity := inst.Severity // TODO: add check, we expect severity to be info
		// TODO: we could do this without a loop
		for k, v := range inst.Variables {
			// fmt.Println(k, ": ", decodeValue(v.GetValue()))
			if k == "cookies" {
				logInstance.Values[k] = v.GetStringValue()
			} else if k == "url" {
				logInstance.Values[k] = v.GetStringValue()
			} else if k == "destinationName" {
				logInstance.Values[k] = v.GetStringValue()
				logInstance.Destination = v.GetStringValue()
			} else if k == "responseCode" {
				logInstance.Values[k] = v.GetInt64Value()
			} else if k == "responseDuration" {
				logInstance.Values[k] = v.GetDurationValue()
			} else if k == "destinationPort" {
				logInstance.Values[k] = v.GetInt64Value()
				logInstance.DestinationPort = v.GetStringValue()
			} else if k == "apiProtocol" {
				logInstance.Values[k] = v.GetStringValue()
			} else if k == "requestMethod" {
				logInstance.Values[k] = v.GetStringValue()
			} else if strings.HasPrefix(k, "label_") {
				labelName := strings.TrimPrefix(k, "label_")
				labelValue := v.GetStringValue()
				if labelValue != "" {
					logInstance.DestinationLabels[labelName] = labelValue
				}
			}
			/*
				TODO: enable this when StringMap type bug fix is merged to istio
				else if k == "destinationLabels" {
					if destinationLabelsTemp, err := cast.ToStringMapStringE(decodeValue(v.GetValue())); err != nil {
						DestinationLabels = destinationLabelsTemp
					} else {
						logging.Error("Cast Error: %v\n", err)
					}
				} */

		}

		processor.LogInstanceChannel <- logInstance
		// fmt.Printf("logInstance: %v\n", logInstance)
	}

	return nil
}

// Addr returns the listening address of the server
func (s *VampAdapter) Addr() string {
	return s.listener.Addr().String()
}

// Run starts the server run
func (s *VampAdapter) Run(shutdown chan error) {
	shutdown <- s.server.Serve(s.listener)
}

// Close gracefully shuts down the server; used for testing
func (s *VampAdapter) Close() error {
	if s.server != nil {
		s.server.GracefulStop()
	}

	if s.listener != nil {
		_ = s.listener.Close()
	}

	return nil
}

func getServerTLSOption(credential, privateKey, caCertificate string) (grpc.ServerOption, error) {
	certificate, err := tls.LoadX509KeyPair(
		credential,
		privateKey,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load key cert pair")
	}
	certPool := x509.NewCertPool()
	bs, err := ioutil.ReadFile(caCertificate)
	if err != nil {
		return nil, fmt.Errorf("failed to read client ca cert: %s", err)
	}

	ok := certPool.AppendCertsFromPEM(bs)
	if !ok {
		return nil, fmt.Errorf("failed to append client certs")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{certificate},
		ClientCAs:    certPool,
	}
	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert

	return grpc.Creds(credentials.NewTLS(tlsConfig)), nil
}

// NewVampAdapter creates a new IBP adapter that listens at provided port.
func NewVampAdapter(addr string) (Server, error) {
	fmt.Printf("Running on port %v\n", addr)
	if addr == "" {
		addr = "0"
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%s", addr))
	if err != nil {
		return nil, fmt.Errorf("unable to listen on socket: %v", err)
	}
	s := &VampAdapter{
		listener: listener,
	}
	fmt.Printf("listening on \"%v\"\n", s.Addr())

	credential := os.Getenv("GRPC_ADAPTER_CREDENTIAL")
	privateKey := os.Getenv("GRPC_ADAPTER_PRIVATE_KEY")
	certificate := os.Getenv("GRPC_ADAPTER_CERTIFICATE")
	if credential != "" {
		so, err := getServerTLSOption(credential, privateKey, certificate)
		if err != nil {
			return nil, err
		}
		fmt.Printf("Starting server with credentials\n")
		s.server = grpc.NewServer(so)
	} else {
		fmt.Printf("Starting server without credentials\n")
		s.server = grpc.NewServer()
	}
	logentry.RegisterHandleLogEntryServiceServer(s.server, s)
	go processor.RunProcessor()
	return s, nil
}
