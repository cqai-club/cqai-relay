// Account Service sends a signed envelope to the generic Task Plugin API.
// The channel key is the server-only InferFlow platform key and the channel
// base URL must include /openapi/v1.  Quotes and material uploads stay in the
// Account Service; this plugin owns only the durable Relay task lifecycle.
// Use exactly one channel key: the Account Service signs the envelope with
// that same platform key, so channel-side multi-key rotation is unsupported.
export const meta = {
  apiVersion: 1,
  key: "ejianbao",
  name: "e剪宝数字人",
  version: "1.1.0",
  author: { name: "CQAI Club" },
  description: {
    en: "Account-authorized digital-human generation",
    zh: "产品账户授权的数字人口播生成",
  },
  models: ["ejianbao-digitalhuman"],
  fetchMode: "per_task",
  auth: { type: "api_key" },
  usageSchema: {
    seconds: {
      type: "number",
      unit: "second",
      description: {
        en: "Billable output seconds, with a ten-second minimum.",
        zh: "按实际输出秒数计费，最低十秒。",
      },
    },
  },
  usageExamples: [{ label: "10s", facts: { seconds: 10 } }],
};

const IDENTIFIER = /^[A-Za-z0-9][A-Za-z0-9._~-]{0,127}$/;
const MODEL = "ejianbao-digitalhuman";
// InferFlow may return an API-relative download route instead of a signed URL.
// Keep the accepted path fixed so output metadata cannot point the proxy at an
// unrelated endpoint or another run.
const RELATIVE_VIDEO_DOWNLOAD = /^\/openapi\/v1\/runs\/([A-Za-z0-9][A-Za-z0-9._~-]{0,127})\/outputs\/video\/download$/;

function identifier(value) {
  if (typeof value !== "string" || !IDENTIFIER.test(value)) throw new Error("invalid identifier");
  return value;
}

function signedInput(ctx) {
  const body = ctx.requestBody || {};
  if (
    typeof body.envelope !== "string" ||
    body.envelope.length === 0 ||
    body.envelope.length > 40000 ||
    typeof body.signature !== "string" ||
    body.signature.length !== 64 ||
    utils.hmacSHA256(body.envelope, ctx.apiKey) !== body.signature
  ) {
    throw new Error("account authorization required");
  }

  let input;
  try {
    input = JSON.parse(body.envelope);
  } catch (_error) {
    throw new Error("invalid signed input");
  }
  if (!input || typeof input !== "object" || Array.isArray(input)) throw new Error("invalid signed input");
  if (body.model !== MODEL) throw new Error("unsupported digital-human model");
  identifier(input.avatar_id);
  identifier(input.voice_id);
  identifier(input.request_id);
  if (
    typeof input.script_text !== "string" ||
    !input.script_text.trim() ||
    input.script_text.length > 5000 ||
    !Number.isInteger(input.seconds) ||
    input.seconds < 10 ||
    input.seconds > 1800
  ) {
    throw new Error("invalid signed input");
  }
  return input;
}

function upstreamAuth(ctx) {
  // The current InferFlow OpenAPI client authenticates every request with
  // X-API-Key. Keep the vendor credential server-side in the Task Plugin
  // channel; it is never part of the account envelope.
  return { "X-API-Key": ctx.apiKey };
}

function outputItems(body) {
  if (!body || typeof body !== "object" || Array.isArray(body)) return [];
  // The public schema currently uses `items`, while a few gateway versions
  // expose the same collection as `outputs`. Read both when present so a
  // manifest in one collection cannot hide the video in the other.
  return [
    ...(Array.isArray(body.items) ? body.items : []),
    ...(Array.isArray(body.outputs) ? body.outputs : []),
  ];
}

function outputURL(item) {
  if (!item || typeof item !== "object" || Array.isArray(item)) return "";
  for (const key of ["download_url", "downloadUrl", "url"]) {
    const value = item[key];
    if (typeof value === "string" && (/^https:\/\//.test(value) || RELATIVE_VIDEO_DOWNLOAD.test(value))) return value;
  }
  return "";
}

function videoOutput(body) {
  return outputItems(body).find((item) => {
    if (!item || typeof item !== "object" || Array.isArray(item)) return false;
    const descriptor = [item.type, item.name, item.format, item.content_type, item.contentType, item.mime_type, item.mimeType]
      .map((value) => String(value || "")).join(" ");
    return Boolean(outputURL(item)) && (item.name === "video" || /video|mp4/i.test(descriptor));
  });
}

function shouldFetchOutputs(previous) {
  const status = String(previous && previous.status || "").toLowerCase();
  return ["completed", "partial_success"].includes(status) && !videoOutput(previous);
}

export function extractUsage(ctx) {
  const input = signedInput(ctx);
  // Tiered billing consumes facts; the separate ratio hook intentionally does
  // not multiply a second time when the model has a usage expression.
  return ctx.usagePurpose === "billing_ratios" ? null : { seconds: input.seconds };
}

export function buildSubmitRequest(ctx) {
  const input = signedInput(ctx);
  return {
    url: ctx.baseUrl + "/skills/digital_human_standard/runs",
    method: "POST",
    headers: Object.assign({}, upstreamAuth(ctx), {
      "Content-Type": "application/json",
      "Idempotency-Key": input.request_id,
    }),
    body: {
      inputs: {
        avatar_id: input.avatar_id,
        voice_id: input.voice_id,
        script_text: input.script_text,
        segmentation_mode: "fast_segments",
      },
    },
  };
}

export function parseSubmitResponse(_ctx, response) {
  if (response.statusCode < 200 || response.statusCode >= 300) throw new Error("digital-human service rejected submission");
  const body = response.body;
  if (!body || typeof body !== "object" || Array.isArray(body)) throw new Error("invalid digital-human submission response");
  return { taskId: identifier(body.run_id), taskData: body };
}

export function buildQueryRequest(ctx) {
  const previous = ctx.requestBody && ctx.requestBody.data;
  const path = shouldFetchOutputs(previous)
    ? "/runs/" + identifier(ctx.taskId) + "/outputs"
    : "/runs/" + identifier(ctx.taskId);
  return {
    url: ctx.baseUrl + path,
    method: "GET",
    headers: upstreamAuth(ctx),
  };
}

export function parseTaskResult(_ctx, body) {
  if (!body || typeof body !== "object" || Array.isArray(body)) throw new Error("invalid digital-human status response");
  // InferFlow exposes the completed files at /runs/:id/outputs, separately
  // from the status document. The host passes the previous response back to
  // buildQueryRequest, so one extra poll can fetch and persist that index.
  const rawStatus = String(body.status || "").toLowerCase();
  const hasOutputCollection = Array.isArray(body.items) || Array.isArray(body.outputs);
  const outputs = outputItems(body);
  if (["failed", "cancelled", "canceled"].includes(rawStatus)) {
    return { status: "FAILURE", progress: "100%", reason: "Digital-human generation failed" };
  }
  // Treat a non-video output (for example, a manifest emitted before the
  // video object) as eventual consistency rather than a permanent failure.
  // The host timeout remains the upper bound if the video never appears.
  if (videoOutput(body)) return { status: "SUCCESS", progress: "100%" };
  if (outputs.length > 0) {
    return { status: "IN_PROGRESS", progress: "99%" };
  }
  // The output index can be briefly empty after the run reaches completed.
  // Keep polling until the normal task timeout instead of failing a valid run.
  if (!rawStatus && hasOutputCollection) return { status: "IN_PROGRESS", progress: "99%" };
  let status;
  if (["completed", "partial_success"].includes(rawStatus)) status = "SUCCESS";
  else if (["queued", "pending", "running", "processing", "in_progress", "submitted", "voice_generating", "video_generating"].includes(rawStatus)) status = "IN_PROGRESS";
  else throw new Error("unknown digital-human task status");
  if (status === "SUCCESS" && !videoOutput(body)) status = "IN_PROGRESS";
  const progress = Number(body.progress_percent);
  const bounded = Number.isFinite(progress) ? Math.max(0, Math.min(100, progress)) : 0;
  return {
    status,
    // A 100% progress value is excluded from the host polling query even when
    // the status is still IN_PROGRESS. Reserve 100% for terminal results.
    progress: (status === "IN_PROGRESS" ? Math.min(99, bounded) : bounded) + "%",
    ...(status === "FAILURE" ? { reason: "Digital-human generation failed" } : {}),
  };
}

export function extractUsageOnComplete(_task, result, body) {
  if (!result || result.status !== "SUCCESS") return null;
  const output = videoOutput(body);
  if (!output) return null;
  const rawSeconds = output.duration_seconds ?? output.duration;
  if (rawSeconds === undefined) return null;
  const seconds = Number(rawSeconds);
  if (!Number.isFinite(seconds) || seconds <= 0 || seconds > 1800) throw new Error("invalid output duration");
  return { seconds: Math.max(10, Math.ceil(seconds)) };
}

export function listArtifacts(task) {
  return task.status === "SUCCESS" && videoOutput(task.data)
    ? [{ key: "video", type: "video", mimeType: "video/mp4" }]
    : [];
}

export function buildContentRequest(ctx) {
  if (ctx.artifactKey !== "video") throw new Error("unknown artifact");
  const url = outputURL(videoOutput(ctx.data));
  if (!url) throw new Error("artifact_not_found");
  const relative = RELATIVE_VIDEO_DOWNLOAD.exec(url);
  if (relative) {
    if (relative[1] !== identifier(ctx.upstreamTaskId)) throw new Error("artifact_not_found");
    const base = /^(https:\/\/[A-Za-z0-9.-]+(?::[0-9]{1,5})?)\/openapi\/v1\/?$/.exec(ctx.baseUrl);
    if (!base) throw new Error("invalid InferFlow base URL");
    return {
      // This API route requires the channel key and returns the video directly.
      // The host validates the same-origin URL, preserves client Range headers,
      // and rejects credentialed cross-origin redirects.
      url: base[1] + url,
      method: (ctx.clientRequest && ctx.clientRequest.method) || "GET",
      headers: upstreamAuth(ctx),
    };
  }
  return {
    // A signed HTTPS object URL must never receive the platform key.
    url,
    method: (ctx.clientRequest && ctx.clientRequest.method) || "GET",
    credentialless: true,
  };
}
