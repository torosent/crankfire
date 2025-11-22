FROM golang:1.24 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o crankfire ./cmd/crankfire

FROM alpine:latest

WORKDIR /root/

COPY --from=builder /app/crankfire .

ENTRYPOINT ["./crankfire"]
