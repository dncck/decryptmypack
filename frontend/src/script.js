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

async function decrypt() {
    let inputElement = document.getElementById("input");
    let input = inputElement.value.trim();
    if (!input) {
        input = inputElement.placeholder.trim();
        inputElement.value = input;
    }
    let apiUrl = apiBaseUrl() + `/download?target=` + encodeURIComponent(input);

    showDownload();
    hideError();

    try {
        let response = await fetch(apiUrl);
        let payload = await readAPIResponse(response);

        if (!response.ok) {
            throw new Error(payload.error || "Download request failed");
        }

        if (payload.status === "ready") {
            triggerDownload(payload.url);
            hideDownload();
            return;
        }

        if (payload.status !== "processing" || !payload.url) {
            throw new Error("Unexpected download response");
        }

        payload = await pollForPack(apiUrl, payload.poll_interval_ms || 3000);
        triggerDownload(payload.url);
    } catch (error) {
        showError("Error downloading file: " + error.message);
    } finally {
        hideDownload();
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
    errorElement.style.opacity = "1"; // Set opacity to 1 to show the element

    setTimeout(hideError, 15000); // Hide the error message after 3 seconds
}

// Function to hide the error message with fade-out effect
function hideError() {
    const errorElement = document.getElementById("error");
    errorElement.style.opacity = "0"; // Set opacity to 0 to hide the element
}

function showDownload() {
    const loadingElement = document.getElementById("loading");
    loadingElement.style.opacity = "1";
}

function hideDownload() {
    const loadingElement = document.getElementById("loading");
    loadingElement.style.opacity = "0";
}

function toDiscord() {
    window.location.href = "https://discord.gg/SqTuFapfWv";
}

function toGithub() {
    window.location.href = "https://github.com/RestartFU/decryptmypack";
}
