build_service:
	@cd service && go build -o ../bin/service cmd/service/main.go
build_handler_example:
	@cd handler_example && CGO_ENABLED=0 go build -o ../bin/handler_example main.go
