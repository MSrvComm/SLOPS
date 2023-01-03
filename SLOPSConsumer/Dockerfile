FROM golang:alpine as builder

WORKDIR /build
COPY . .

RUN apk add git
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -a -installsuffix cgo -ldflags '-extldflags "-static"' -o consumer ./cmd

FROM scratch

COPY --from=builder /build/consumer /app/
WORKDIR /app
CMD ["./consumer"]