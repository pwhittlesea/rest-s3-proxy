FROM golang:1.5
MAINTAINER Phillip Whittlesea <pw.github@thga.me.uk>

ADD . /go/src/github.com/pwhittlesea/rest-s3-proxy

RUN go get -u -v github.com/aws/aws-sdk-go/...

RUN go install github.com/pwhittlesea/rest-s3-proxy

RUN rm -rf /go/src /go/pkg

CMD ["/go/bin/rest-s3-proxy"]
EXPOSE 8000
