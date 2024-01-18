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

# xds-status-intermediary shows the current xDS resources ACKed by greeter-intermediary
.PHONY: xds-status-intermediary
xds-status-intermediary:
	grpcdebug localhost:50055 xds status

# xds-status-leaf shows the current xDS resources ACKed by greeter-leaf
.PHONY: xds-status-leaf
xds-status-leaf:
	grpcdebug localhost:50057 xds status

# xds-config-intermediary dumps the current xDS configuration of greeter-intermediary
.PHONY: xds-config-intermediary
xds-config-intermediary:
	grpcdebug localhost:50055 xds config | yq --input-format=json --prettyPrint

# xds-config-leaf dumps the current xDS configuration of greeter-leaf
.PHONY: xds-config-leaf
xds-config-leaf:
	grpcdebug localhost:50057 xds config | yq --input-format=json --prettyPrint

# delete the greeter-intermediary and greeter-leaf Deployment and Service resources from the k8s cluster.
.PHONY: delete
delete:
	kubectl delete --ignore-not-found --namespace=xds deployment greeter-intermediary greeter-leaf
	kubectl delete --ignore-not-found --namespace=xds service greeter-intermediary greeter-leaf

.PHONY: build
build:
	../gradlew assembleDist

.PHONY: clean
clean:
	../gradlew clean

.PHONY: depupdates
depupdates:
	../gradlew dependencyUpdates

.PHONY: format
format:
	../gradlew googleJavaFormat

.PHONY: image
image:
	skaffold build --detect-minikube=false --skip-tests

.PHONY: imagejib
imageko:
	../gradlew jibDockerBuild

.PHONY: lint
lint: format
	../gradlew verifyGoogleJavaFormat checkstyleMain spotbugsMain

.PHONY: manifests
manifests:
	skaffold render --digest-source=local --loud=false

.PHONY: proto
proto:
	../gradlew generateProto

.PHONY: test
test:
	../gradlew test
