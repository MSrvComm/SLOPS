all: producer consumer client

producer:
	echo Building producer
	cd ./SLOPSProducer && sudo ./build.sh

consumer:
	echo Building consumer
	cd ./SLOPSConsumer && sudo ./build.sh

client:
	cd ./SLOPSClient && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o SLOPSClient ./cmd