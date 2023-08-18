FROM golang:1.21.0

RUN mkdir -p /go/metadata-updater
WORKDIR /go/metadata-updater
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY main.go .
RUN go build -tags lambda.norpc -o bootstrap main.go

FROM amazon/aws-lambda-provided:al2.2023.05.29.17

RUN yum update -y && yum install -y createrepo_c rpm-sign && rm -rf /var/cache/yum/* && yum clean all

COPY --from=0 /go/metadata-updater/bootstrap /var/runtime/bootstrap
CMD [ "metadata-updater" ]
