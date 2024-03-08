# Copyright 2023 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

PKG := github.com/googlecloudplatform/solutions-workshops/grpc-xds/control-plane-go

# Set global Skaffold options
export SKAFFOLD_BUILD_CONCURRENCY:=0
export SKAFFOLD_CACHE_ARTIFACTS:=false
export SKAFFOLD_CLEANUP:=false
export SKAFFOLD_DETECT_MINIKUBE:=false
export SKAFFOLD_INTERACTIVE:=false
export SKAFFOLD_SKIP_TESTS:=true
export SKAFFOLD_UPDATE_CHECK:=false

.PHONY: all
all: clean proto format lint test

# run builds and deploys the xDS control plane, sets up port forwarding, and tails the logs.
.PHONY: run
run:
	skaffold run --port-forward=user --tail

# debug builds and deploys the xDS control plane, and sets up remote debugging.
.PHONY: debug
debug:
	skaffold debug --port-forward=user

# dev builds and deploys the xDS control plane, and sets up file watching for automatic image
# rebuilds and redeploys on source code changes.
.PHONY: dev
dev:
	skaffold dev

# tail the logs of the xDS control plane.
.PHONY: tail
tail:
	kubectl logs --all-containers --follow --namespace=xds deployment/control-plane

# delete the xds control plane Deployment and Service resources from the k8s cluster.
.PHONY: delete
delete:
	kubectl delete --ignore-not-found --namespace=xds deployment control-plane
	kubectl delete --ignore-not-found --namespace=xds service control-plane
	kubectl delete --ignore-not-found --namespace=xds configmaps --selector="app.kubernetes.io/part-of"=grpc-xds

.PHONY: build
build:
	CGO_ENABLED=0 go build $(PKG)

.PHONY: clean
clean:
	go clean -i $(PKG)/...

.PHONY: deps
deps:
	go mod download

.PHONY: format
format:
	goimports -e -l -local $(PKG) -w .

.PHONY: image
image:
	skaffold build

.PHONY: imageko
imageko:
	ko build --base-import-paths --local --sbom=none

.PHONY: lint
lint: format
	golangci-lint -v run

.PHONY: manifests
manifests:
	skaffold render --digest-source=local --loud=false

.PHONY: proto
proto:
	go generate $(PKG)/...

.PHONY: test
test:
	go test -count 1 -race -timeout 3m -v $(PKG)/...

.PHONY: updatedeps
updatedeps:
	go get -d -t -u $(PKG)/...
