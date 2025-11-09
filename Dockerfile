FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o analyzer cmd/main.go

FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /root/
COPY --from=builder /app/analyzer .
COPY static ./static
EXPOSE 8080
ENV PORT=8080
ENV REDIS_ADDR=redis:6379
CMD ["./analyzer"]