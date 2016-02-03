FROM ubuntu
MAINTAINER Phillip Whittlesea <pw.github@thga.me.uk>

RUN apt-get update
RUN apt-get install -y ca-certificates

COPY app /app
CMD ["/app"]
