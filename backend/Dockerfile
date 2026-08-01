FROM golang:1.25-alpine AS build

ARG BUILD_VERSION=dev
ARG BUILD_COMMIT=unknown
ARG BUILD_TIME=unknown

WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags="-s -w -X main.buildVersion=${BUILD_VERSION} -X main.buildCommit=${BUILD_COMMIT} -X main.buildTime=${BUILD_TIME}" \
    -o /out/campus-api ./cmd/server

FROM alpine:3.20

WORKDIR /app
RUN adduser -D -H appuser
COPY --from=build /out/campus-api /app/campus-api
USER appuser
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8080/readyz || exit 1
CMD ["/app/campus-api"]
