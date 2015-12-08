FROM ubuntu:15.10

RUN apt-get update -qq \
  && apt-get install pkg-config libsodium-dev golang git -y

ENV GOPATH /go
WORKDIR /go/src/github.com/Tox
RUN git clone https://github.com/Tox/ToxStatus
WORKDIR /go/src/github.com/Tox/ToxStatus
RUN go get
ENTRYPOINT /go/bin/ToxStatus

EXPOSE 8081
