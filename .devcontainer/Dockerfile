ARG GO_VERSION=1.26.3
ARG ALPINE_VERSION=3.23

FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION}

# Install packages
RUN apk add -q --update --progress --no-cache \
    git \
    make \ 
    openssh-client \
    sudo