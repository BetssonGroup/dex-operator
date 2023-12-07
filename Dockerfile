# Build the manager binary
FROM --platform=$BUILDPLATFORM container-registry.test.betsson.tech/cache/library/golang:1.14 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY apis/ apis/
COPY controllers/ controllers/
COPY pkg/ pkg/

# Build
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -a -o dex-operator main.go

# Use upx to reduce the docker image size
FROM container-registry.test.betsson.tech/cache/hairyhenderson/upx:3.94 as upx
COPY --from=builder /workspace/dex-operator /workspace/dex-operator
RUN upx -9 /workspace/dex-operator

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
ARG BUILD_DATE
ARG SOURCE_COMMIT
WORKDIR /
COPY --from=upx /workspace/dex-operator .
USER nonroot:nonroot

ENTRYPOINT ["/dex-operator"]
