# =============================================================================
#                     Butaq Compiler Makefile
# =============================================================================

.PHONY: all build clean test help

# Output binary name
BINARY=butaq

ifeq ($(OS),Windows_NT)
    BINARY_EXT=.exe
else
    BINARY_EXT=
endif

TARGET_BINARY=$(BINARY)$(BINARY_EXT)

all: build

build:
	@echo "🔧 Building Butaq Compiler..."
	go build -o $(TARGET_BINARY) main.go
	@echo "✅ Built: ./$(TARGET_BINARY)"

test:
	@echo "🏃 Running automated regression tests..."
	go run scripts/test_runner.go

clean:
	@echo "🧹 Cleaning up intermediate object and binary files..."
	rm -f *.o *.obj *.exe out.asm $(TARGET_BINARY)
	@echo "✨ Clean completed!"

help:
	@echo "Butaq Compiler Makefile targets:"
	@echo "  make build  - Compile the Butaq compiler binary"
	@echo "  make test   - Run the automated example test suite"
	@echo "  make clean  - Clean up build artifacts and residues"
