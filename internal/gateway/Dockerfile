FROM golang:alpine AS builder
WORKDIR /app
COPY internal/gateway/go.mod internal/gateway/go.sum ./
RUN go mod download
COPY internal/gateway/ .
RUN go build -o gateway .

FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/
COPY --from=builder /app/gateway .
EXPOSE 8080
CMD ["./gateway"]
