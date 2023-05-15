FROM golang:latest

WORKDIR /op-coordinator

COPY go.mod go.sum /op-coordinator
RUN go mod download
COPY . .

RUN go build -o coordinator .

CMD ["./coordinator"]
