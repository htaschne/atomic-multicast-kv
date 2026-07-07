FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN go build -o /out/atomic-multicast-kv .

FROM alpine:3.20

RUN adduser -D -H appuser
USER appuser
WORKDIR /app
COPY --from=build /out/atomic-multicast-kv /app/atomic-multicast-kv

ENTRYPOINT ["/app/atomic-multicast-kv"]
