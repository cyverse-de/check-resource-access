FROM golang:1.17-alpine

RUN apk add --no-cache git
RUN go get -u github.com/jstemmer/go-junit-report

WORKDIR /build

COPY . .
ENV CGO_ENABLED=0
RUN go install -v ./...

ENTRYPOINT ["check-resource-access"]

ARG git_commit=unknown
ARG version="2.9.0"
ARG descriptive_version=unknown

LABEL org.cyverse.git-ref="$git_commit"
LABEL org.cyverse.version="$version"
LABEL org.cyverse.descriptive-version="$descriptive_version"
LABEL org.label-schema.vcs-ref="$git_commit"
LABEL org.label-schema.vcs-url="https://github.com/cyverse-de/check-resource-access"
LABEL org.label-schema.version="$descriptive_version"
