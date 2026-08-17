ARG GO_VERSION=1.26.6
ARG ALPINE_VERSION=3.24

FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION}

# Install packages
RUN apk add -q --update --progress --no-cache \
    git \
    make \ 
    openssh-client \
    sudo