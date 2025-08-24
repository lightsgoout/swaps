FROM golang:1.25.0-alpine as builder

ENV CGO_ENABLED=0

WORKDIR /work

COPY go.mod go.sum ./
RUN go mod download -x && go mod verify

COPY . .
RUN go build -o /go/bin/swapper-api cmd/swapper-api/main.go
RUN go build -o /go/bin/swapper-worker cmd/swapper-worker/main.go
RUN go build -o /go/bin/producer cmd/producer/main.go

FROM scratch
#COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
#COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=builder /go/bin/swapper-api /go/bin/swapper-api
COPY --from=builder /go/bin/swapper-worker /go/bin/swapper-worker
COPY --from=builder /go/bin/producer /go/bin/producer

ENTRYPOINT ["/go/bin/swapper-api"]
