FROM golang:1.25.0

RUN mkdir -p /go/metadata-updater
WORKDIR /go/metadata-updater
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY main.go .
ENV CGO_ENABLED=0
RUN go build -tags lambda.norpc -o bootstrap main.go

FROM public.ecr.aws/lambda/provided:al2.2023.09.29.09

RUN yum update -y && yum install -y createrepo_c rpm-sign && rm -rf /var/cache/yum/* && yum clean all

COPY --from=0 /go/metadata-updater/bootstrap /var/runtime/bootstrap
CMD [ "metadata-updater" ]
