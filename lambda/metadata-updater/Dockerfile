FROM golang:1.27.1

RUN mkdir -p /go/metadata-updater
WORKDIR /go/metadata-updater
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY main.go .
ENV CGO_ENABLED=0
RUN go build -tags lambda.norpc -o bootstrap main.go

FROM public.ecr.aws/lambda/provided:al2023.2026.09.22.03

RUN dnf update -y && dnf install -y createrepo_c rpm-sign && rm -rf /var/cache/dnf/* && dnf clean all

COPY --from=0 /go/metadata-updater/bootstrap /var/runtime/bootstrap
CMD [ "metadata-updater" ]
