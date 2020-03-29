package pgo_cli_test

/*
 Copyright 2020 Crunchy Data Solutions, Inc.
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

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/crunchydata/postgres-operator/testing/kubeapi"
)

func TestMain(m *testing.M) {
	must := func(b bool, message string, args ...interface{}) {
		if !b {
			panic(fmt.Errorf(message, args...))
		}
	}

	os.Exit(func() int {
		{
			config, err := kubeapi.NewConfig()
			must(err == nil, "kubernetes config: %v", err)

			// Nothing's gonna stop us now.
			config.QPS = 1000.0
			config.Burst = 2000.0

			TestContext.Kubernetes, err = kubeapi.NewForConfig(config)
			must(err == nil, "kubernetes client: %v", err)
		}

		// By default, use a port-forward proxy to talk to the Operator.
		if url := os.Getenv("PGO_APISERVER_URL"); url == "" {
			if ns := os.Getenv("PGO_OPERATOR_NAMESPACE"); ns != "" {
				pods, err := TestContext.Kubernetes.ListPods(ns, map[string]string{"name": "postgres-operator"})
				must(err == nil, "list pods: %v", err)
				must(len(pods) > 0, "missing postgres-operator")

				port := "8443"
				for _, c := range pods[0].Spec.Containers {
					if c.Name == "apiserver" {
						must(len(c.Ports) > 0, "missing proxy port")
						port = fmt.Sprintf("%d", c.Ports[0].ContainerPort)
					}
				}

				proxy, err := TestContext.Kubernetes.PodPortForward(pods[0].Namespace, pods[0].Name, port)
				must(err == nil, "pod port forward: %v", err)
				defer proxy.Close()

				TestContext.DefaultEnvironment = append(TestContext.DefaultEnvironment,
					"PGO_APISERVER_URL=https://"+proxy.LocalAddr(),
				)
			}
		}

		// By default, use files that are generated by the Ansible installer.
		if ns := os.Getenv("PGO_OPERATOR_NAMESPACE"); ns != "" {
			if home, err := os.UserHomeDir(); err == nil {
				TestContext.DefaultEnvironment = append(TestContext.DefaultEnvironment,
					"PGO_CA_CERT="+filepath.Join(home, ".pgo", ns, "output", "server.crt"),
					"PGO_CLIENT_CERT="+filepath.Join(home, ".pgo", ns, "output", "server.crt"),
					"PGO_CLIENT_KEY="+filepath.Join(home, ".pgo", ns, "output", "server.pem"),
				)
			}
		}

		if scale := os.Getenv("PGO_TEST_TIMEOUT_SCALE"); scale != "" {
			s, _ := strconv.ParseFloat(scale, 64)
			must(s > 0, "PGO_TEST_TIMEOUT_SCALE must be a fractional number greater than zero")
			TestContext.Scale = func(d time.Duration) time.Duration { return time.Duration(s * float64(d)) }
		} else {
			TestContext.Scale = func(d time.Duration) time.Duration { return d }
		}

		return m.Run()
	}())
}

var TestContext struct {
	// DefaultEnvironment specifies environment variables to be passed to every
	// executed process. Each entry is of the form "key=value", and values in
	// os.Environ() take precedence. See https://golang.org/pkg/os/exec/#Cmd.
	DefaultEnvironment []string

	Kubernetes *kubeapi.KubeAPI

	Scale func(time.Duration) time.Duration
}
