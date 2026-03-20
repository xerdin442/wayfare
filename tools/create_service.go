package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	serviceName := flag.String("name", "", "Name of the service")
	flag.Parse()

	if *serviceName == "" {
		fmt.Println("Please provide a service name using -name flag")
		os.Exit(1)
	}

	// Create service directory structure
	basePath := filepath.Join("services", *serviceName)
	dirs := []string{
		"cmd",
		"internal/models",
		"internal/service",
		"internal/infra/events",
		"internal/infra/grpc",
		"internal/infra/repository",
		"pkg/types",
	}

	for _, dir := range dirs {
		fullPath := filepath.Join(basePath, dir)
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			fmt.Printf("Error creating directory %s: %v\n", dir, err)
			os.Exit(1)
		}
	}

	fmt.Printf("Successfully created %s service structure in %s\n", *serviceName, basePath)
	fmt.Println("\nDirectory structure created:")
	fmt.Printf(`
services/%s/
├── cmd/                  # Application entry points
├── internal/             # Private application code
│   ├── models/           # Business domain models and interfaces
│   │   └── %s.go         # Core domain interfaces
│   ├── service/          # Business logic implementation
│   │   └── service.go    # Service implementations
│   └── infra/   					# External dependencies implementations (abstractions)
│       ├── events/       # Event handling (RabbitMQ)
│       ├── grpc/         # gRPC server handlers
│       └── repository/   # Data persistence
└── pkg/                  # Public packages
    └── types/           	# Shared types and models
`, *serviceName, *serviceName)
}
