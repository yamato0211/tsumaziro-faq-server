ARG GO_VERSION=1.22
ARG ALPINE_VERSION=3.18

FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION} as go-builder

WORKDIR /go/src/tsumaziro-faq-server

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN go build -o api main.go

FROM alpine:${ALPINE_VERSION}

WORKDIR /usr/src/tsumaziro-faq-server

COPY --from=go-builder /go/src/tsumaziro-faq-server/api api
COPY --from=datadog/serverless-init:1 /datadog-init /app/datadog-init
ENTRYPOINT ["/app/datadog-init"]
ENV DD_SERVICE=datadog-demo-run-go
ENV DD_ENV=datadog-demo
ENV DD_VERSION=1

RUN chmod +x "/usr/src/tsumaziro-faq-server/api"

CMD ["/usr/src/tsumaziro-faq-server/api"]