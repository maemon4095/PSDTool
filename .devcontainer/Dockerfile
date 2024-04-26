ARG NODE_VERSION=18.20.2
ARG ALPINE_VERSION=3.19.1
ARG GO_VERSION=1.17

FROM node:${NODE_VERSION}-alpine AS node
FROM golang:${GO_VERSION}-alpine AS go
FROM alpine:${ALPINE_VERSION}

COPY --from=node /usr/lib /usr/lib
COPY --from=node /usr/local/lib /usr/local/lib
COPY --from=node /usr/local/include /usr/local/include
COPY --from=node /usr/local/bin /usr/local/bin

COPY --from=go /usr/local/go/ /usr/local/go/
ENV PATH="/usr/local/go/bin:${PATH}"
ENV PATH="/root/go/bin:${PATH}"
RUN go install github.com/gopherjs/gopherjs@v1.17.2

RUN apk update && apk add git python3 make build-base