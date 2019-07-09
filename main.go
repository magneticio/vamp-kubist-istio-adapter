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

package main

import (
	"fmt"
	"os"

	vampadapter "github.com/magneticio/vamp-kubist-istio-adapter/adapter"
	"github.com/magneticio/vampkubistcli/logging"
	"github.com/spf13/viper"
)

func main() {
	viper.SetEnvPrefix("vamp")
	viper.BindEnv("url")
	viper.BindEnv("token")
	viper.BindEnv("apiversion")
	viper.BindEnv("cert")
	viper.BindEnv("project")
	viper.BindEnv("cluster")
	viper.BindEnv("virtualcluster")
	viper.BindEnv("logging")
	viper.SetDefault("logging", "verbose")

	fmt.Printf("URL: %v\n", viper.GetString("url"))

	if viper.GetString("logging") == "verbose" {
		logging.Verbose = true
	}
	logging.Init(os.Stdout, os.Stderr)
	// This is kept for testing
	// "/tmp/documentation1/vamp-config.yaml"
	// configurator.InitViperConfig("/tmp/documentation1", "vamp-config")
	addr := "9000"
	if len(os.Args) > 1 {
		addr = os.Args[1]
	}

	s, err := vampadapter.NewVampAdapter(addr)
	if err != nil {
		fmt.Printf("unable to start server: %v", err)
		os.Exit(-1)
	}

	shutdown := make(chan error, 1)
	go func() {
		s.Run(shutdown)
	}()
	_ = <-shutdown
}
