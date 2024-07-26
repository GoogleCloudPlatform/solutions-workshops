// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"net"
	"net/http"

	"github.com/go-logr/logr"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func listenHTTPHealth(logger logr.Logger, listener net.Listener, healthServer *health.Server) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		service := r.URL.Query().Get("service") // optional
		logger.Info("Received HTTP request", "url", r.URL.String(), "service", service, "remoteAddr", r.RemoteAddr, "headers", r.Header)
		healthResp, err := healthServer.Check(r.Context(), &healthpb.HealthCheckRequest{
			Service: service,
		})
		if err != nil {
			logger.Error(err, "could not access health status", "service", service)
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(service))
			return
		}
		healthStatusCode := healthResp.GetStatus()
		healthStatusName := healthpb.HealthCheckResponse_ServingStatus_name[int32(healthStatusCode)]
		if healthStatusCode == healthpb.HealthCheckResponse_SERVING {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(healthStatusName))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(healthStatusName))
	})
	httpHealthServer := &http.Server{Handler: h2c.NewHandler(mux, &http2.Server{})}
	return httpHealthServer.Serve(listener)
}
