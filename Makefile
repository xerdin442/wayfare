PROTO_DIR := shared/proto
PROTO_SRC := $(wildcard $(PROTO_DIR)/*.proto)
PROTO_INCLUDE = C:/Users/HP/AppData/Local/Microsoft/WinGet/Packages/Google.Protobuf_Microsoft.Winget.Source_8wekyb3d8bbwe/include
GO_OUT := shared/pkg

.PHONY: compile-proto
compile-proto:
	protoc \
		-I=$(PROTO_DIR) \
		-I=$(PROTO_INCLUDE) \
		--go_out=$(GO_OUT) \
		--go_opt=paths=source_relative \
		--go-grpc_out=$(GO_OUT) \
		--go-grpc_opt=paths=source_relative \
		$(PROTO_SRC)