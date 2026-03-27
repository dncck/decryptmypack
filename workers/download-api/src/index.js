export default {
    async fetch(request, env) {
        if (request.method === "OPTIONS") {
            return new Response(null, { status: 204, headers: corsHeaders() });
        }

        const url = new URL(request.url);
        if (url.pathname !== "/download" && url.pathname !== "/health") {
            return json({ error: "not found" }, 404);
        }

        if (url.pathname === "/download" && request.method !== "GET") {
            return json({ error: "method not allowed" }, 405, {
                Allow: "GET, OPTIONS",
            });
        }

        if (url.pathname === "/health" && request.method !== "GET") {
            return json({ error: "method not allowed" }, 405, {
                Allow: "GET, OPTIONS",
            });
        }

        let packRequest = null;
        if (url.pathname === "/download") {
            packRequest = await parsePackRequest(url);
            if (packRequest.error) {
                return json({ status: "error", error: packRequest.error }, 400);
            }

            const packState = await getPackState(env, packRequest.stateKey);
            if (packState?.status === "error") {
                return json(
                    {
                        status: "error",
                        error:
                            packState.error ||
                            "recent download failure; try again later",
                    },
                    429,
                );
            }

            const cachedObject = await getCachedPack(
                env,
                packRequest.objectKey,
            );
            if (cachedObject) {
                return json(
                    {
                        status: "ready",
                        key: packRequest.objectKey,
                        url: publicPackURL(env, packRequest.objectKey),
                    },
                    200,
                );
            }

            if (packState?.status === "running") {
                return json(
                    {
                        status: "processing",
                        key: packRequest.objectKey,
                        url: publicPackURL(env, packRequest.objectKey),
                        poll_interval_ms: 3000,
                    },
                    202,
                );
            }
        }

        const upstream = new URL(
            url.pathname,
            normalizeOrigin(env.DOWNLOAD_API_ORIGIN),
        );
        upstream.search = url.search;

        const headers = new Headers();
        headers.set("X-Download-Api-Secret", env.DOWNLOAD_API_SHARED_SECRET);
        const cfConnectingIP = request.headers.get("CF-Connecting-IP");
        if (cfConnectingIP) {
            headers.set("X-Forwarded-For", cfConnectingIP);
        }

        let response;
        try {
            response = await fetch(upstream, {
                method: request.method,
                headers,
            });
        } catch (error) {
            if (
                url.pathname === "/download" &&
                packRequest &&
                !packRequest.error
            ) {
                await setPackState(
                    env,
                    packRequest.stateKey,
                    {
                        status: "error",
                        error: `origin unavailable: ${error.message}`,
                    },
                    2 * 60,
                );
            }
            return json(
                {
                    status: "error",
                    error: `origin unavailable: ${error.message}`,
                },
                502,
            );
        }

        const responseHeaders = new Headers(response.headers);
        applyCors(responseHeaders);

        if (url.pathname === "/download" && response.ok) {
            const responseClone = response.clone();
            const payload = await responseClone.json();

            if (payload.status === "processing") {
                await setPackState(
                    env,
                    packRequest.stateKey,
                    { status: "running" },
                    30,
                );
            } else {
                await clearPackState(env, packRequest.stateKey);
            }
        }
        if (url.pathname === "/download" && !response.ok) {
            const responseClone = response.clone();
            const errorMessage = await extractErrorMessage(responseClone);
            await setPackState(
                env,
                packRequest.stateKey,
                {
                    status: "error",
                    error: errorMessage,
                },
                2 * 60,
            );
        }

        return new Response(response.body, {
            status: response.status,
            statusText: response.statusText,
            headers: responseHeaders,
        });
    },
};

function normalizeOrigin(origin) {
    return origin.replace(/\/+$/, "");
}

async function getPackState(env, objectKey) {
    if (!env.FAILED_PACKS) {
        return null;
    }

    const raw = await env.FAILED_PACKS.get(objectKey, "json");
    return raw || null;
}

async function setPackState(env, objectKey, payload, ttlSeconds) {
    if (!env.FAILED_PACKS) {
        return;
    }

    await env.FAILED_PACKS.put(objectKey, JSON.stringify(payload), {
        expirationTtl: ttlSeconds,
    });
}

async function clearPackState(env, objectKey) {
    if (!env.FAILED_PACKS) {
        return;
    }

    await env.FAILED_PACKS.delete(objectKey);
}

async function getCachedPack(env, objectKey) {
    if (!env.PACKS) {
        return null;
    }

    const object = await env.PACKS.head(objectKey);
    if (!object) {
        return null;
    }

    return object;
}

async function parsePackRequest(url) {
    const rawTarget = (url.searchParams.get("target") || "").trim();
    if (!rawTarget) {
        return { error: "missing target" };
    }

    const split = rawTarget.split(":");
    let target = split[0].trim().toLowerCase();
    let port = "19132";

    if (target.includes("hivebedrock.network")) {
        target = "geo.hivebedrock.network";
    }

    if (!target) {
        return { error: "missing target" };
    }

    if (split.length > 1) {
        if (!/^\d+$/.test(split[1])) {
            return { error: "invalid port" };
        }
        port = split[1];
    }

    const resolvedIP = await resolveTargetIP(target);
    if (!resolvedIP) {
        return { error: "invalid target" };
    }

    return {
        target,
        port,
        objectKey: `packs/${target}/${port}/${target}.zip`,
        stateKey: `ip/${resolvedIP.toLowerCase()}/${port}`,
    };
}

function publicPackURL(env, objectKey) {
    const base = (env.PACKS_PUBLIC_BASE_URL || "").replace(/\/+$/, "");
    if (!base) {
        return `/${objectKey}`;
    }
    return `${base}/${objectKey}`;
}

async function extractErrorMessage(response) {
    const contentType = response.headers.get("content-type") || "";
    if (contentType.includes("application/json")) {
        try {
            const payload = await response.json();
            return (
                payload.error || `request failed with status ${response.status}`
            );
        } catch {
            return `request failed with status ${response.status}`;
        }
    }

    const text = (await response.text()).trim();
    return text || `request failed with status ${response.status}`;
}

async function resolveTargetIP(target) {
    if (isIPAddress(target)) {
        return target;
    }

    const ipv4 = await resolveViaDOH(target, "A");
    if (ipv4) {
        return ipv4;
    }
    return await resolveViaDOH(target, "AAAA");
}

async function resolveViaDOH(target, type) {
    const response = await fetch(
        `https://cloudflare-dns.com/dns-query?name=${encodeURIComponent(target)}&type=${type}`,
        {
            headers: {
                Accept: "application/dns-json",
            },
        },
    );
    if (!response.ok) {
        return "";
    }

    const payload = await response.json();
    const answers = Array.isArray(payload.Answer) ? payload.Answer : [];
    const record = answers.find(
        (answer) =>
            answer.type === (type === "A" ? 1 : 28) &&
            typeof answer.data === "string",
    );
    return record?.data || "";
}

function isIPAddress(value) {
    return /^(\d{1,3}\.){3}\d{1,3}$/.test(value) || value.includes(":");
}

function corsHeaders() {
    return new Headers({
        "access-control-allow-origin": "*",
        "access-control-allow-methods": "GET, OPTIONS",
        "access-control-allow-headers": "Content-Type",
    });
}

function applyCors(headers) {
    for (const [name, value] of corsHeaders()) {
        headers.set(name, value);
    }
}

function json(payload, status, extraHeaders = {}) {
    const headers = corsHeaders();
    headers.set("content-type", "application/json");
    for (const [name, value] of Object.entries(extraHeaders)) {
        headers.set(name, value);
    }
    return new Response(JSON.stringify(payload), { status, headers });
}
