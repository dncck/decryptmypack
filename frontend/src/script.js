function updateInputValue(input) {
    let inputElement = document.getElementById("input");
    inputElement.value = input;
}

function downloadSpecifiedServer(addr) {
    updateInputValue(addr);
    decrypt();
}

function apiBaseUrl() {
    let configured = window.DECRYPTMYPACK_CONFIG?.apiBaseUrl || "";
    return configured.replace(/\/+$/, "");
}

function setStatus(state, title, text, target = "") {
    const card = document.getElementById("status-card");
    const dot = document.getElementById("status-dot");
    const titleElement = document.getElementById("status-title");
    const textElement = document.getElementById("status-text");
    const targetElement = document.getElementById("status-target");

    card.dataset.state = state;
    dot.dataset.state = state;
    titleElement.textContent = title;
    textElement.textContent = text;
    targetElement.textContent = target;
}

function setBusy(isBusy) {
    const button = document.getElementById("decrypt-btn");
    button.disabled = isBusy;
    button.textContent = isBusy ? "decrypting..." : "decrypt pack";
}

async function decrypt() {
    let inputElement = document.getElementById("input");
    let input = inputElement.value.trim();
    if (!input) {
        input = inputElement.placeholder.trim();
        inputElement.value = input;
    }
    let apiUrl = apiBaseUrl() + `/download?target=` + encodeURIComponent(input);

    setBusy(true);
    hideError();
    setStatus(
        "loading",
        "Checking cache",
        "Looking for an existing pack in cache.",
        `Target: ${input}`,
    );

    try {
        let response = await fetch(apiUrl);
        let payload = await readAPIResponse(response);

        if (!response.ok) {
            throw new Error(payload.error || "Download request failed");
        }

        if (payload.status === "ready") {
            setStatus(
                "success",
                "Cached pack found",
                "Starting the download now.",
                `Target: ${input}`,
            );
            triggerDownload(payload.url);
            return;
        }

        if (payload.status !== "processing" || !payload.url) {
            throw new Error("Unexpected download response");
        }

        setStatus(
            "loading",
            "Generating pack",
            "Connecting to the server, decrypting, and uploading the result.",
            `Target: ${input}`,
        );
        payload = await pollForPack(apiUrl, payload.poll_interval_ms || 3000);
        setStatus(
            "success",
            "Pack ready",
            "Download is starting now.",
            `Target: ${input}`,
        );
        triggerDownload(payload.url);
    } catch (error) {
        showError("Error downloading file: " + error.message);
        setStatus(
            "error",
            "Download failed",
            error.message,
            `Target: ${input}`,
        );
    } finally {
        setBusy(false);
    }
}

async function readAPIResponse(response) {
    let contentType = response.headers.get("content-type") || "";
    if (contentType.includes("application/json")) {
        return await response.json();
    }

    let text = (await response.text()).trim();
    return {
        error: text || `Request failed with status ${response.status}`,
    };
}

async function pollForPack(apiUrl, intervalMs) {
    let deadline = Date.now() + 5 * 60 * 1000;
    while (Date.now() < deadline) {
        let response = await fetch(apiUrl);
        let payload = await readAPIResponse(response);
        if (!response.ok) {
            throw new Error(
                payload.error ||
                    `Request failed with status ${response.status}`,
            );
        }
        if (payload.status === "ready" && payload.url) {
            return payload;
        }
        if (payload.status !== "processing") {
            throw new Error(payload.error || "Unexpected download response");
        }
        setStatus(
            "loading",
            "Still working",
            "The pack is not cached yet. The backend is still processing it.",
        );
        await new Promise((resolve) => setTimeout(resolve, intervalMs));
    }
    throw new Error("Timed out waiting for the pack to upload");
}

function triggerDownload(url) {
    let a = document.createElement("a");
    a.href = url;
    a.rel = "noopener";
    document.body.appendChild(a);
    a.click();
    a.remove();
}

// Function to show the error message with fade-in effect
function showError(err) {
    const errorElement = document.getElementById("error");
    errorElement.textContent = err;
    errorElement.style.opacity = "1";

    setTimeout(hideError, 15000);
}

function hideError() {
    const errorElement = document.getElementById("error");
    errorElement.style.opacity = "0";
}

function toDiscord() {
    window.location.href = "https://discord.gg/YDaxHMySd7";
}

function toGithub() {
    window.location.href = "https://github.com/RestartFU/decryptmypack";
}

setStatus("idle", "Ready", "Enter a server and start a download.");
