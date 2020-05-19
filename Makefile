# Image URL to use all building/pushing image targets
CRD_GROUP ?= dex.betssongroup.com
IMG ?= "quay.io/betsson-oss/dex-operator"
TAG=$(shell git symbolic-ref -q --short HEAD)
GIT_SHA1=$(shell git rev-parse -q HEAD)
BUILD_DATE=$(shell date +%FT%T%Z)
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Kind
KIND_NAME ?= dex
KIND_VERSION ?= v1.16.9

# Registry options
REG_NAME ?= kind-registry
REG_PORT ?= 5000


# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

# Run tests
test: generate fmt vet manifests
	go test ./... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./main.go

# Install CRDs into a cluster
install: manifests
	kustomize build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests
	kustomize build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	cd config/manager && \
		kustomize edit set image controller=${IMG}:${TAG} && \
		kustomize edit add annotation -f app.kubernetes.io/version:${TAG}
	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the docker image
docker-build: test
	docker build --build-arg SOURCE_COMMIT=${GIT_SHA1} --build-arg BUILD_DATE=${BUILD_DATE} . -t ${IMG}:${TAG}

# Push the docker image
docker-push: kind-registry
	docker push ${IMG}:${TAG}

kind-registry:
	set -o errexit ;\
	running="$$(docker inspect -f '{{.State.Running}}' $(REG_NAME) 2>/dev/null || true)" ;\
	if [ "$${running}" != 'true' ]; then \
	docker run -d --restart=always -p "$(REG_PORT):$(REG_PORT)" --name $(REG_NAME) "registry:2" ;\
	fi

kind-up: kind-registry
	docker pull kindest/node:$(KIND_VERSION) ;\
	if [ ! $$(kind get clusters -q | grep -q $(KIND_NAME)) ]; then \
		kind create cluster --name $(KIND_NAME) --config contrib/kind/kind.yaml --image=kindest/node:$(KIND_VERSION) --wait 120s ;\
	else \
		echo "Kind cluster $(KIND_NAME) already running" ;\
	fi ;\
	docker network connect "kind" $(REG_NAME) 2>/dev/null;\
	kubectl cluster-info --context kind-$(KIND_NAME) ;\
	kind export kubeconfig --name $(KIND_NAME) ;\
	for node in $$(kind get nodes --name $(KIND_NAME)); do \
		kubectl annotate --overwrite node "$${node}" "kind.x-k8s.io/registry=localhost:$(REG_PORT)" ;\
	done

kind-delete:
	kind delete cluster --name $(KIND_NAME)

deploy-ingress:
	kind export kubeconfig --name $(KIND_NAME) && \
	kubectl apply -f contrib/static/ingress-nginx/deploy.yaml && \
	kubectl wait --namespace ingress-nginx \
	--for=condition=ready pod \
	--selector=app.kubernetes.io/component=controller \
	--timeout=90s && \
	echo "Ingress nginx setup and ready"

deploy-cert-manager: deploy-ingress
	kind export kubeconfig --name $(KIND_NAME) && \
	kubectl apply -f contrib/static/cert-manager/ns.yaml && \
	if ! helm list -n cert-manager --deployed -q|grep -q cert-manager; then \
		cd contrib/charts/cert-manager && helm install cert-manager --namespace cert-manager --version v0.15.0 --set installCRDs=true . ;\
	fi

deploy-prometheus-operator:
	@kind export kubeconfig --name $(KIND_NAME)
	kubectl apply -f contrib/static/prometheus-operator/bundle.yaml

deploy-dex: deploy-prometheus-operator deploy-cert-manager
	kind export kubeconfig --name $(KIND_NAME) && \
	kubectl apply -f contrib/static/dex/ns.yaml && \
	if ! helm list -n dex --deployed -q|grep -q dex; then \
		cd contrib/charts/dex && helm install dex --namespace dex .;\
	fi

refresh: docker-build docker-push
	kubectl -n dex rollout restart deployment dex-operator-controller-manager

in-cluster-test:
	@echo "Cleanup"
	kubectl -n dex delete clients.$(CRD_GROUP) test-client; true 
	kubectl -n default delete clients.$(CRD_GROUP) test-client; true
	kubectl -n dex delete clients.$(CRD_GROUP) test-minimal-client; true
	kubectl -n dex delete oauth2clients.dex.coreos.com --all; true
	sleep 2
	@kubectl -n dex apply -f config/samples/oauth2_v1_client.yaml
	sleep 2
	@kubectl -n dex describe clients.$(CRD_GROUP) test-client
	sleep 2
	@kubectl -n default apply -f config/samples/oauth2_v1_client.yaml
	sleep 2
	@kubectl -n default describe clients.$(CRD_GROUP) test-client
	@kubectl -n dex apply -f config/samples/minimal.yaml
	sleep 1
	@kubectl -n dex describe oauth2clients.dex.coreos.com
	@kubectl -n dex describe clients.$(CRD_GROUP)

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.5 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif
