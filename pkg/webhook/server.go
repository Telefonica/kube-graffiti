/*
Copyright (C) 2018 Expedia Group.
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

package webhook

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"net/url"

	"github.com/Telefonica/kube-graffiti/pkg/graffiti"
	"github.com/Telefonica/kube-graffiti/pkg/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	componentName = "webhook"
	pathPrefix    = "/graffiti/"
)

type Server struct {
	CompanyDomain string
	Namespace     string
	Service       string
	CACert        []byte
	httpServer    *http.Server
	handler       graffitiHandler
}

// NewServer creates a new webhook server and sets up the initial graffiti handler.
// Use AddGraffitiRule to load the rules into the webhook server before starting.
func NewServer(cd, ns, svc string, ca []byte, k *kubernetes.Clientset, port int) Server {
	mylog := log.ComponentLogger(componentName, "NewServer")
	mylog.Debug().Int("port", port).Msg("creating a new webhook server")

	mylog.Debug().Msg("creating a new http mux")
	mux := http.NewServeMux()
	mylog.Info().Msg("configuring http tls configuration")
	tls := configTLS(k)
	server := &http.Server{
		Addr:      fmt.Sprintf(":%d", port),
		Handler:   mux,
		TLSConfig: tls,
	}
	return Server{
		CompanyDomain: cd,
		Namespace:     ns,
		Service:       svc,
		CACert:        ca,
		httpServer:    server,
		handler:       newGraffitiHandler(),
	}
}

// AddGraffitiRule provides a way of adding new rules into the http mux and corresponding handler context map
func (s Server) AddGraffitiRule(rule graffiti.Rule) {
	path := pathFromName(rule.Name)
	mux := s.httpServer.Handler.(*http.ServeMux)
	mux.Handle(path, s.handler)
	s.handler.addRule(path, rule)
}

// StartWebhookServer starts the webhook server with TLS encryption
func (s Server) StartWebhookServer(certPath, keyPath string) {
	mylog := log.ComponentLogger(componentName, "StartWebhookSecureServer")
	mylog.Debug().Str("certPath", certPath).Str("keyPath", keyPath).Msg("starting the secure webhook http server...")

	// start the webhook server in a new routine
	go func() {
		if err := s.httpServer.ListenAndServeTLS(certPath, keyPath); err != nil {
			mylog.Fatal().Err(err).Msg("failed to start the webhook server")
		}
	}()

	return
}

func configTLS(clientset *kubernetes.Clientset) *tls.Config {
	mylog := log.ComponentLogger(componentName, "configTLS")
	mylog.Debug().Msg("calling kubernetes api to retrieve the CA certificate")

	cert := getAPIServerCert(clientset)
	apiserverCA := x509.NewCertPool()
	apiserverCA.AppendCertsFromPEM(cert)

	return &tls.Config{
		ClientCAs:  apiserverCA,
		ClientAuth: tls.NoClientCert,
		NextProtos: []string{"http/1.1"},
	}
}

// retrieve the CA cert that will signed the cert used by the
// "GenericAdmissionWebhook" plugin admission controller.
func getAPIServerCert(clientset *kubernetes.Clientset) []byte {
	mylog := log.ComponentLogger(componentName, "getAPIServerCert")
	mylog.Debug().Msg("Starting getAPIServerCert")
	c, err := clientset.CoreV1().ConfigMaps(metav1.NamespaceSystem).Get("extension-apiserver-authentication", metav1.GetOptions{})
	if err != nil {
		mylog.Fatal().Err(err).Msg("failed to read the extension-apiserver-authentication configmap")
	}

	pem, ok := c.Data["requestheader-client-ca-file"]
	if !ok {
		mylog.Fatal().Msg(fmt.Sprintf("cannot find the ca.crt in the configmap, configMap.Data is %#v", c.Data))
	}
	mylog.Debug().Str("client-ca", pem).Msg("client ca loaded")
	return []byte(pem)
}

func pathFromName(name string) string {
	mylog := log.ComponentLogger(componentName, "Path")
	path := pathPrefix + url.PathEscape(name)
	mylog.Debug().Str("path", path).Msg("Generated webhook path")
	return path
}
