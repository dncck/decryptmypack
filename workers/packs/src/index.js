export default {
  async fetch(request, env) {
    if (request.method === "OPTIONS") {
      return new Response(null, { status: 204, headers: corsHeaders() });
    }

    const url = new URL(request.url);
    const key = url.pathname.replace(/^\/+/, "");

    if (!key.startsWith("packs/")) {
      return json({ error: "not found" }, 404);
    }

    if (request.method !== "GET" && request.method !== "HEAD") {
      return json({ error: "method not allowed" }, 405, {
        Allow: "GET, HEAD, OPTIONS",
      });
    }

    const object = await env.PACKS.get(key);
    if (!object) {
      return json({ error: "not found" }, 404);
    }

    const headers = corsHeaders();
    object.writeHttpMetadata(headers);
    headers.set("etag", object.httpEtag);
    headers.set("last-modified", object.uploaded.toUTCString());
    headers.set("cache-control", headers.get("cache-control") || "public, max-age=86400");
    headers.set("content-type", headers.get("content-type") || "application/zip");
    headers.set("content-disposition", headers.get("content-disposition") || contentDisposition(key));

    if (request.method === "HEAD") {
      headers.set("content-length", object.size.toString());
      return new Response(null, { headers });
    }

    return new Response(object.body, { headers });
  },
};

function corsHeaders() {
  return new Headers({
    "access-control-allow-origin": "*",
    "access-control-allow-methods": "GET, HEAD, OPTIONS",
    "access-control-expose-headers": "Content-Length, Content-Disposition, Content-Type, ETag, Last-Modified",
  });
}

function contentDisposition(key) {
  const filename = key.split("/").pop() || "pack.zip";
  return `attachment; filename="${filename}"`;
}

function json(payload, status, extraHeaders = {}) {
  const headers = corsHeaders();
  headers.set("content-type", "application/json");
  for (const [name, value] of Object.entries(extraHeaders)) {
    headers.set(name, value);
  }
  return new Response(JSON.stringify(payload), { status, headers });
}
