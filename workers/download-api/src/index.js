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

        if (url.pathname === "/download") {
            const packRequest = parsePackRequest(url);
            if (packRequest.error) {
                return json({ status: "error", error: packRequest.error }, 400);
            }

            const cooldown = await getFailedPackCooldown(
                env,
                packRequest.objectKey,
            );
            if (cooldown) {
                return json(
                    {
                        status: "error",
                        error:
                            cooldown.error ||
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
            if (url.pathname === "/download") {
                const packRequest = parsePackRequest(url);
                if (!packRequest.error) {
                    await setFailedPackCooldown(
                        env,
                        packRequest.objectKey,
                        `origin unavailable: ${error.message}`,
                    );
                }
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
            await clearFailedPackCooldown(env, parsePackRequest(url).objectKey);
        }
        if (url.pathname === "/download" && !response.ok) {
            const responseClone = response.clone();
            const errorMessage = await extractErrorMessage(responseClone);
            await setFailedPackCooldown(
                env,
                parsePackRequest(url).objectKey,
                errorMessage,
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

async function getFailedPackCooldown(env, objectKey) {
    if (!env.FAILED_PACKS) {
        return null;
    }

    const raw = await env.FAILED_PACKS.get(objectKey, "json");
    return raw || null;
}

async function setFailedPackCooldown(env, objectKey, error) {
    if (!env.FAILED_PACKS) {
        return;
    }

    await env.FAILED_PACKS.put(objectKey, JSON.stringify({ error }), {
        expirationTtl: 30 * 60,
    });
}

async function clearFailedPackCooldown(env, objectKey) {
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

function parsePackRequest(url) {
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

    return {
        target,
        port,
        objectKey: `packs/${target}/${port}/${target}.zip`,
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
