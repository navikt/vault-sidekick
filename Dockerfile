FROM golang:1.24 as builder
ENV GOOS=linux
ENV GOARCH=amd64
ENV CGO_ENABLED=0
COPY . /src
WORKDIR /src
RUN go get
RUN go test ./...
RUN go build -a -installsuffix cgo -o vault-sidekick

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /src/vault-sidekick /vault-sidekick

ENTRYPOINT [ "/vault-sidekick" ]
