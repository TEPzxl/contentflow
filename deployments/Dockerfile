FROM golang:1.25-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates git

ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY}

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/contentflow ./cmd/server

FROM alpine:3.21

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /out/contentflow /app/contentflow
COPY configs /app/configs

EXPOSE 8080

ENV CONTENTFLOW_CONFIG=/app/configs/config.docker.yaml

ENTRYPOINT ["/app/contentflow"]
