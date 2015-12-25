FROM busybox
MAINTAINER Phillip Whittlesea <pw.github@thga.me.uk>
COPY app /app
CMD ["/app"]
