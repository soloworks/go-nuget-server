FROM golang:1.13.3-alpine3.10 as builder

WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -mod=readonly -v -o server

FROM alpine:3.10
RUN apk add --no-cache ca-certificates

COPY --from=builder /app/server /server
COPY nuget-server-config-gcp.json /

CMD ["/server"]
