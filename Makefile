.PHONY: run build tidy clean

# Remove artefatos
clean:
	rm -f gastos gastos.db

# Build para produção (Linux arm64)
build-linux:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=1 go build -ldflags="-s -w" -o gastos-linux ./src/main.go