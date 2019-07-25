FROM golang:1.12-alpine as builder
RUN apk add --no-cache git
ENV GOOS=linux
ENV CGO_ENABLED=0
ENV GO111MODULE=on
COPY . /src
WORKDIR /src
RUN rm -f go.sum
RUN go get
RUN go test ./...
RUN go build -a -installsuffix cgo -o vault-sidekick

FROM alpine:3.9
RUN apk update && \
    apk add ca-certificates bash
RUN adduser -D vault

COPY --from=builder /src/vault-sidekick /vault-sidekick
RUN chmod 755 /vault-sidekick

USER vault

ENTRYPOINT [ "/vault-sidekick" ]
