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
    let go;
    try {
        go = new Go();
        const wasmUrl = "progressbar.wasm?v=" + Date.now();
        const result = await WebAssembly.instantiateStreaming(
            fetch(wasmUrl),
            go.importObject
        );
        // run is intentionally not awaited — it resolves when the Go program exits
        go.run(result.instance);
    } catch (err) {
        showError("Failed to load WASM: " + err.message);
        return;
    }

    // 2. Wait for bridge functions to be registered by Go
    function bridgeReady() {
        return (
            typeof globalThis.bubbletea_write === "function" &&
            typeof globalThis.bubbletea_read === "function" &&
            typeof globalThis.bubbletea_resize === "function"
        );
    }

    await new Promise(function (resolve, reject) {
        if (bridgeReady()) {
            resolve();
            return;
        }
        let attempts = 0;
        const maxAttempts = 100; // 5 seconds at 50ms
        const id = setInterval(function () {
            attempts++;
            if (bridgeReady()) {
                clearInterval(id);
                resolve();
            } else if (attempts >= maxAttempts) {
                clearInterval(id);
                reject(new Error("Timed out waiting for WASM bridge functions"));
            }
        }, 50);
    }).catch(function (err) {
        showError(err.message);
        throw err;
    });

    // 3. Create xterm.js terminal
    const term = new Terminal({
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

    const fitAddon = new FitAddon.FitAddon();
    term.loadAddon(fitAddon);
    term.open(document.getElementById("terminal"));

    // WebGL renderer draws block/box characters with vector math
    // instead of font glyphs — pixel-perfect Unicode rendering.
    try {
        const webglAddon = new WebglAddon.WebglAddon();
        term.loadAddon(webglAddon);
    } catch (e) {
        console.warn("WebGL addon failed, falling back to DOM renderer:", e);
    }

    fitAddon.fit();
    term.focus();

    // Hide the loading indicator
    loadingEl.classList.add("hidden");

    // 4. Send initial terminal size to Go
    bubbletea_resize(term.cols, term.rows);

    // 5. Wire up the bridges

    // Keyboard input -> Go
    term.onData(function (data) {
        bubbletea_write(data);
    });

    // Enter alternate screen buffer — fixed canvas, no scrollback.
    // This is how BubbleTea renders in a real terminal too.
    term.write("\x1b[?1049h");

    // Go output -> terminal. Renders one complete frame at a time.
    var pending = false;
    function pollOutput() {
        if (pending) return;
        var frame = bubbletea_read();
        if (frame && frame.length > 0) {
            pending = true;
            term.write("\x1b[H" + frame + "\x1b[J", function () {
                pending = false;
            });
        }
    }
    setInterval(pollOutput, 100);

    // Terminal resize -> Go
    term.onResize(function (size) {
        bubbletea_resize(size.cols, size.rows);
    });

    // Window resize -> fit terminal
    window.addEventListener("resize", function () {
        fitAddon.fit();
    });
})();
