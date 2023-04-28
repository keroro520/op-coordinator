# Build Geth in a stock Go builder container
FROM golang:1.19-alpine as builder

RUN apk add --no-cache gcc musl-dev linux-headers git

# Get dependencies - will also be cached if we won't change go.mod/go.sum
COPY go.mod /coordinator-cli/
COPY go.sum /coordinator-cli/
RUN cd /coordinator-cli && go mod download

ADD . /coordinator-cli
RUN cd /coordinator-cli && go build -o coordinator-cli main.go

# Pull Geth into a second stage deploy alpine container
FROM alpine:latest

RUN apk add --no-cache ca-certificates
COPY --from=builder /coordinator-cli/coordinator-cli /usr/local/bin

ENTRYPOINT ["coordinator-cli"]
