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

# The `--build-concurrency=0`, `--detect-minikube=false`, and `--skip-tests`
# flags in the commands below are optional. They are included only to speed up
# the commands.

# run-go builds and deploys control-plane-go and greeter-go.
.PHONY: run-go
run-go: check-skaffold check-go
	skaffold run --build-concurrency=0 --detect-minikube=false --module=go --port-forward=user --skip-tests

# run-java builds and deploys control-plane-java and greeter-java.
.PHONY: run-java
run-java: check-skaffold check-javac
	skaffold run --build-concurrency=0 --detect-minikube=false --module=java --port-forward=user --skip-tests

# debug-go builds and deploys control-plane-go and greeter-go, with remote debugging enabled.
.PHONY: debug-go
debug-go: check-skaffold check-go
	skaffold debug --build-concurrency=0 --cleanup=false --detect-minikube=false --module=go --port-forward=user --skip-tests

# debug-java builds and deploys control-plane-java and greeter-java, with remote debugging enabled.
.PHONY: debug-java
debug-java: check-skaffold check-javac
	skaffold debug --build-concurrency=0 --cleanup=false --detect-minikube=false --module=java --port-forward=user --skip-tests

# dev-go sets up file watching with automatic image rebuilds and redeploys for control-plane-go and greeter-go.
.PHONY: dev-go
dev-go: check-skaffold check-go
	skaffold dev --build-concurrency=0 --cleanup=false --detect-minikube=false --module=go --skip-tests

# dev-java sets up file watching with automatic image rebuilds and redeploys for control-plane-java and greeter-java.
.PHONY: dev-java
dev-java: check-skaffold check-javac
	skaffold dev --build-concurrency=0 --cleanup=false --detect-minikube=false --module=java --skip-tests

# request sends a request to the greeter-intermediary server. Port forwarding must be running.
# Verify that the port in the command below matches the local forwarded port.
.PHONY: request
request: check-grpcurl
	grpcurl -plaintext -d '{"name": "World"}' localhost:50055 helloworld.Greeter/SayHello

# request-leaf sends a request to the greeter-leaf server. Port forwarding must be running.
# Verify that the port in the command below matches the local forwarded port.
.PHONY: request-leaf
request-leaf: check-grpcurl
	grpcurl -plaintext -d '{"name": "World"}' localhost:50057 helloworld.Greeter/SayHello

# tail-control-plane tails the control-plane logs
.PHONY: tail-control-plane
tail-control-plane: check-kubectl
	kubectl logs --all-containers --follow --namespace=xds deployment/control-plane

# tail-greeter-intermediary tails the greeter-intermediary logs
.PHONY: tail-greeter-intermediary
tail-greeter-intermediary: check-kubectl
	kubectl logs --all-containers --follow --namespace=xds deployment/greeter-intermediary

# tail-greeter-intermediary tails the greeter-leaf logs
.PHONY: tail-greeter-leaf
tail-greeter-leaf: check-kubectl
	kubectl logs --all-containers --follow --namespace=xds deployment/greeter-leaf

# xds-status-intermediary shows the current xDS resources ACKed by greeter-intermediary
.PHONY: xds-status-intermediary
xds-status-intermediary: check-grpcdebug
	grpcdebug localhost:50055 xds status

# xds-status-leaf shows the current xDS resources ACKed by greeter-leaf
.PHONY: xds-status-leaf
xds-status-leaf: check-grpcdebug
	grpcdebug localhost:50057 xds status

# xds-config-intermediary dumps the current xDS configuration of greeter-intermediary
.PHONY: xds-config-intermediary
xds-config-intermediary: check-grpcdebug check-yq
	grpcdebug localhost:50055 xds config | yq --input-format=json --prettyPrint

# xds-config-leaf dumps the current xDS configuration of greeter-leaf
.PHONY: xds-config-leaf
xds-config-leaf: check-grpcdebug check-yq
	grpcdebug localhost:50057 xds config | yq --input-format=json --prettyPrint

# run-bastion deploys a bastion Pod with troubleshooting tools.
.PHONY: run-bastion
run-bastion: check-skaffold
	skaffold run --detect-minikube=false --module=troubleshoot --skip-tests

# troubleshoot executes a shell in the bastion Pod, in raw terminal mode.
.PHONY: troubleshoot
troubleshoot: check-kubectl
	kubectl exec --container=app --namespace=xds --stdin --tty deployment/bastion -- /bin/sh

# delete-control-plane deletes the Services and Deployments for control-plane-[go|java].
.PHONY: delete-control-plane
delete-control-plane:
	kubectl delete --ignore-not-found --namespace=xds deployment control-plane
	kubectl delete --ignore-not-found --namespace=xds service control-plane

# delete-greeter deletes the Services and Deployments for greeter-[intermediary|leaf].
.PHONY: delete-greeter
delete-greeter:
	kubectl delete --ignore-not-found --namespace=xds deployment greeter-intermediary greeter-leaf
	kubectl delete --ignore-not-found --namespace=xds service greeter-intermediary greeter-leaf

# delete deletes the Services and Deployments for control-plane and greeter-[intermediary|leaf],
# but leaves the bastion Pod running.
.PHONY: delete
delete: delete-control-plane delete-greeter

# clean removes running instances of control-plane-[go|java], greeter-[go|java], and bastion.
.PHONY: clean
clean:
	skaffold delete --detect-minikube=false --module=go
	skaffold delete --detect-minikube=false --module=java
	skaffold delete --detect-minikube=false --module=troubleshoot

.PHONY: check-go
check-go:
	@if ! which go > /dev/null; then \
		echo "error: go is not installed: https://go.dev/doc/install" >&2; \
		exit 1; \
	fi

.PHONY: check-grpcdebug
check-grpcdebug:
	@if ! which grpcdebug > /dev/null; then \
		echo "error: grpcdebug is not installed: https://github.com/grpc-ecosystem/grpcdebug#installation" >&2; \
		exit 1; \
	fi

.PHONY: check-grpcurl
check-grpcurl:
	@if ! which grpcurl > /dev/null; then \
		echo "error: grpcurl is not installed: https://github.com/fullstorydev/grpcurl#installation" >&2; \
		exit 1; \
	fi

.PHONY: check-javac
check-javac:
	@if ! which javac > /dev/null; then \
		echo "error: javac is not installed: https://adoptium.net/" >&2; \
		exit 1; \
	fi

.PHONY: check-kubectl
check-kubectl:
	@if ! which kubectl > /dev/null; then \
		echo "error: kubectl is not installed: https://kubernetes.io/docs/tasks/tools/#kubectl" >&2; \
		exit 1; \
	fi

.PHONY: check-kustomize
check-kustomize:
	@if ! which kustomize > /dev/null; then \
		echo "error: kustomize is not installed: https://kubectl.docs.kubernetes.io/installation/kustomize/" >&2; \
		exit 1; \
	fi

.PHONY: check-skaffold
check-skaffold: check-kubectl check-kustomize
	@if ! which skaffold > /dev/null; then \
		echo "error: skaffold is not installed: https://skaffold.dev/docs/install/" >&2; \
		exit 1; \
	fi

.PHONY: check-yq
check-yq:
	@if ! which yq > /dev/null; then \
		echo "error: yq is not installed: https://github.com/mikefarah/yq#install" >&2; \
		exit 1; \
	fi
