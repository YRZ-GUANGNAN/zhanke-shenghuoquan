FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/campus-api ./cmd/server

FROM alpine:3.20

WORKDIR /app
RUN adduser -D -H appuser
COPY --from=build /out/campus-api /app/campus-api
USER appuser
EXPOSE 8080
CMD ["/app/campus-api"]
