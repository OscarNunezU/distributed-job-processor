proto:
	mkdir -p query-service/gen/job/v1
	protoc \
		--proto_path=proto \
		--go_out=query-service/gen/job/v1 \
		--go_opt=paths=source_relative \
		--go-grpc_out=query-service/gen/job/v1 \
		--go-grpc_opt=paths=source_relative \
		proto/job.proto
