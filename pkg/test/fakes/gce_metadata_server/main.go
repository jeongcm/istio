// Copyright Istio Authors
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
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/mux"
)

const (
	projID     = "test-project"
	projNumber = "123456789"
	instance   = "test-instance"
	instID     = "987654321"
	zone       = "us-west1-c"

	metaPrefix     = "/computeMetadata/v1"
	projIDPath     = metaPrefix + "/project/project-id"
	projNumberPath = metaPrefix + "/project/numeric-project-id"
	instIDPath     = metaPrefix + "/instance/id"
	instancePath   = metaPrefix + "/instance/name"
	zonePath       = metaPrefix + "/instance/zone"
	attrKey        = "attribute"
	attrPath       = metaPrefix + "/instance/attributes/{" + attrKey + "}"
)

var instAttrs = map[string]string{
	"instance-template": "some-template",
	"created-by":        "some-creator",
	"cluster-name":      "test-cluster",
}

var vmInstAttrs = map[string]string{
	"instance-template": "some-template",
	"created-by":        "some-creator",
}

func checkMetadataHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("request for: " + r.URL.Path)
		w.Header().Add("Server", "Metadata Server for VM (Fake)")
		w.Header().Add("Metadata-Flavor", "Google")

		flavor := r.Header.Get("Metadata-Flavor")
		if flavor == "" && r.RequestURI != "/" {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			w.Header().Set("Content-Type", "text/html; charset=UTF-8")
			log.Println("forbidden: " + r.URL.Path)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func handleAttrs(attrs map[string]string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)

		if val, ok := attrs[vars[attrKey]]; ok {
			fmt.Fprint(w, val)
			return
		}

		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	}
}

func main() {
	s1 := runServer(":8080", instAttrs)
	s2 := runServer(":8081", vmInstAttrs)
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	log.Println("GCE metadata server started")
	<-done
	if err := s1.Shutdown(context.Background()); err != nil {
		log.Fatalf("GCE Metadata Shutdown Failed: %+v", err)
	}
	if err := s2.Shutdown(context.Background()); err != nil {
		log.Fatalf("GCE Metadata Shutdown Failed: %+v", err)
	}
}

func runServer(addr string, attrs map[string]string) *http.Server {
	r := mux.NewRouter()
	r.Use(checkMetadataHeaders)
	r.HandleFunc(projIDPath, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, projID) }).Methods("GET")
	r.HandleFunc(projNumberPath, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, projNumber) }).Methods("GET")
	r.HandleFunc(instIDPath, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, instID) }).Methods("GET")
	r.HandleFunc(instancePath, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, instance) }).Methods("GET")
	r.HandleFunc(zonePath, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, zone) }).Methods("GET")
	r.HandleFunc(attrPath, handleAttrs(attrs)).Methods("GET")

	srv := &http.Server{Addr: addr, Handler: r}

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("listen: %v\n", err)
		}
	}()
	return srv
}
