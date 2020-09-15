FROM debian:stretch AS bootstrap

RUN apt-get -qq update && \
        apt-get -qq -y install \
            build-essential \
            curl \
            git

RUN mkdir -p /build/go1.15
# Curl gunzips for us
RUN curl -Ls https://golang.org/dl/go1.15.2.linux-amd64.tar.gz \
        | tar -C /build/go1.15 -xzf -

WORKDIR /build

RUN git clone https://go.googlesource.com/go

WORKDIR /build/go

RUN git checkout -b release-branch.go1.15 origin/release-branch.go1.15

WORKDIR /build/go/src

RUN GOROOT_BOOTSTRAP=/build/go1.15/go GOROOT_FINAL=/go ./all.bash

FROM debian:stretch

RUN mkdir -p /go/bin /go/src/github.com/saymedia/journald-cloudwatch-logs
COPY --from=bootstrap /build/go /go

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get -qq update && apt-get -qq -y install \
            gcc \
            libsystemd-dev \
            git

RUN apt-get clean
RUN rm -rf /var/cache/apt/lists

ENV PATH=${PATH}:/go/bin GOBIN=/go/bin
RUN go get -u github.com/go-delve/delve/cmd/dlv

RUN mkdir /journald-cloudwatch-logs

WORKDIR /journald-cloudwatch-logs
