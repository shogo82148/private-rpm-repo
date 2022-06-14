FROM golang:1.18.3

RUN mkdir -p /go/metadata-updater
WORKDIR /go/metadata-updater
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY main.go .
RUN go build -tags lambda.norpc -o bootstrap main.go

FROM almalinux:9.0-20220527

RUN dnf update -y && dnf install -y createrepo_c rpm-sign && rm -rf /var/cache/dnf/* && dnf clean all

COPY --from=0 /go/metadata-updater/bootstrap /var/runtime/bootstrap
ENTRYPOINT [ "/var/runtime/bootstrap" ]
