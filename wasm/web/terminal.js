(async function () {
    "use strict";

    const loadingEl = document.getElementById("loading");
    const errorEl = document.getElementById("error");

    function showError(msg) {
        loadingEl.classList.add("hidden");
        errorEl.style.display = "block";
        errorEl.textContent = msg;
    }

    // 1. Load and start Go WASM
    try {
        var go = new Go();
        var wasmUrl = "progressbar.wasm?v=" + Date.now();
        var result = await WebAssembly.instantiateStreaming(
            fetch(wasmUrl),
            go.importObject
        );
        go.run(result.instance);
    } catch (err) {
        showError("Failed to load WASM: " + err.message);
        return;
    }

    // 2. Wait for bridge functions
    function bridgeReady() {
        return (
            typeof globalThis.bubbletea_write === "function" &&
            typeof globalThis.bubbletea_read === "function" &&
            typeof globalThis.bubbletea_resize === "function"
        );
    }

    await new Promise(function (resolve, reject) {
        if (bridgeReady()) { resolve(); return; }
        var attempts = 0;
        var id = setInterval(function () {
            attempts++;
            if (bridgeReady()) { clearInterval(id); resolve(); }
            else if (attempts >= 100) { clearInterval(id); reject(new Error("Timed out waiting for WASM bridge")); }
        }, 50);
    }).catch(function (err) { showError(err.message); throw err; });

    // 3. Create xterm.js terminal
    var term = new Terminal({
        theme: {
            background: "#1a1a2e",
            foreground: "#e0e0e0",
            cursor: "#e0e0e0",
        },
        fontFamily: 'Menlo, Monaco, "Courier New", monospace',
        fontSize: 14,
        cursorBlink: false,
        allowProposedApi: true,
    });

    var fitAddon = new FitAddon.FitAddon();
    term.loadAddon(fitAddon);
    term.open(document.getElementById("terminal"));

    try {
        var webglAddon = new WebglAddon.WebglAddon();
        term.loadAddon(webglAddon);
    } catch (e) {
        console.warn("WebGL addon failed, falling back to DOM renderer:", e);
    }

    fitAddon.fit();
    term.focus();
    loadingEl.classList.add("hidden");

    // 4. Send initial terminal size
    bubbletea_resize(term.cols, term.rows);

    // 5. Wire up I/O — same pattern as BigJk/bubbletea-in-wasm:
    // Input is push, output is polled at 100ms.

    term.onData(function (data) {
        bubbletea_write(data);
    });

    setInterval(function () {
        var data = bubbletea_read();
        if (data && data.length > 0) {
            term.write(data);
        }
    }, 100);

    term.onResize(function (size) {
        bubbletea_resize(size.cols, size.rows);
    });

    window.addEventListener("resize", function () {
        fitAddon.fit();
    });
})();
