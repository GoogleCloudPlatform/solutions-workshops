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

PKG := github.com/googlecloudplatform/solutions-workshops/grpc-xds-workshop/control-plane-go

.PHONY: all
all: clean proto format lint test

# run builds and deploys both greeter-intermediary and greeter-leaf, sets up port forwarding, and tails the logs.
.PHONY: run
run:
	skaffold run --detect-minikube=false --port-forward=user --skip-tests --tail

# run-no-xds builds and deploys both greeter-intermediary and greeter-leaf with no xDS configuration.
.PHONY: run-no-xds
run-no-xds:
	skaffold run --detect-minikube=false --port-forward=user --profile=no-xds --skip-tests --tail

# debug builds and deploys both greeter-intermediary and greeter-leaf, and sets up remote debugging.
.PHONY: debug
debug:
	skaffold debug --cleanup=false --detect-minikube=false --port-forward=user --skip-tests

# dev builds and deploys both greeter-intermediary and greeter-leaf, and sets up file watching for
# automatic image rebuilds and redeployments on source code changes.
.PHONY: dev
dev:
	skaffold dev --cleanup=false --detect-minikube=false --skip-tests

# tail the logs of both greeter-intermediary and greeter-leaf.
.PHONY: tail
tail:
	kubectl logs --all-containers --follow --namespace=xds --selector="app.kubernetes.io/name in (greeter-intermediary, greeter-leaf)"

# tail-intermediary tails only the greeter-intermediary logs (all containers).
.PHONY: tail-intermediary
tail-intermediary:
	kubectl logs --all-containers --follow --namespace=xds deployment/greeter-intermediary

# tail-intermediary tails only the greeter-leaf logs (all containers).
.PHONY: tail-leaf
tail-leaf:
	kubectl logs --all-containers --follow --namespace=xds deployment/greeter-leaf

# delete the greeter-intermediary and greeter-leaf Deployment and Service resources from the k8s cluster.
.PHONY: delete
delete:
	kubectl delete --ignore-not-found --namespace=xds {deployment,service}/greeter-{intermediary,leaf}

.PHONY: build
build:
	./gradlew assembleDist

.PHONY: clean
clean:
	./gradlew clean

.PHONY: depupdates
updatedeps:
	./gradlew dependencyUpdates

.PHONY: format
format:
	./gradlew googleJavaFormat

.PHONY: image
image:
	skaffold build --detect-minikube=false --skip-tests

.PHONY: imagejib
imageko:
	./gradlew jibDockerBuild

.PHONY: lint
lint: format
	./gradlew verifyGoogleJavaFormat checkstyleMain spotbugsMain

.PHONY: manifests
manifests:
	skaffold render --digest-source=local --loud=false

.PHONY: proto
proto:
	./gradlew generateProto

.PHONY: test
test:
	./gradlew test
