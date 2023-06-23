FROM golang:alpine as builder

RUN mkdir /build
WORKDIR /build
ADD . /build/

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o gateway ./cmd

FROM scratch

COPY --from=builder /build/gateway /app/
WORKDIR /app
EXPOSE 62000
CMD ["./gateway"]