PROTO_DIR := shared/proto
PROTO_SRC := $(wildcard $(PROTO_DIR)/*.proto)
GO_OUT := shared/pkg

.PHONY: compile-proto
compile-proto:
	protoc \
		-I=$(PROTO_DIR) \
		--go_out=$(GO_OUT) \
		--go_opt=paths=source_relative \
		--go-grpc_out=$(GO_OUT) \
		--go-grpc_opt=paths=source_relative \
		$(PROTO_SRC)