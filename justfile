# Run an example
example name:
    go run ./examples/{{name}}

# Run tests
test:
    go test ./...

# Build WASM demo
wasm:
    GOOS=js GOARCH=wasm go build -ldflags="-s -w" -o wasm/web/progressbar.wasm ./wasm/
    cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" wasm/web/

# Serve WASM demo locally
serve: wasm
    cd wasm/web && python3 -m http.server 8080

# Deploy to GitHub Pages (builds into docs/ for GH Pages)
deploy: wasm
    rm -rf docs/demo
    mkdir -p docs/demo
    cp wasm/web/* docs/demo/
