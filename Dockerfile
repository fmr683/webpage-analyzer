FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download && CGO_ENABLED=0 go build -o analyzer cmd/main.go

FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /root/
COPY --from=builder /app/analyzer .
COPY static ./static
EXPOSE 8080
CMD ["./analyzer"]