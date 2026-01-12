FROM golang:1.24 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Enable CGO for sherpa-onnx
ENV CGO_ENABLED=1
ENV GOOS=linux

RUN go build -o sherpa-server .

FROM debian:bookworm-slim

WORKDIR /app

# Install certificates and potentially other runtime deps
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/sherpa-server /app/sherpa-server

EXPOSE 8081

CMD ["/app/sherpa-server"]
