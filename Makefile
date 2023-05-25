all: producer consumer client controller gateway

producer:
	@echo Building producer
	@cd ./SLOPSProducer && sudo ./build.sh

consumer:
	@echo Building consumer
	@cd ./SLOPSConsumer && sudo ./build.sh

controller:
	@echo Building controller
	@cd ./SLOPSController && sudo ./build.sh

gateway:
	@echo Building gateway
	@cd ./SLOPSGW && sudo ./build.sh

client:
	@cd ./SLOPSClient && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o SLOPSClient ./cmd