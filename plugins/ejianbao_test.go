package plugins_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	builtinplugins "github.com/QuantumNous/new-api/plugins"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ejianbaoSignature(message, secret string) string {
	digest := hmac.New(sha256.New, []byte(secret))
	_, _ = digest.Write([]byte(message))
	return hex.EncodeToString(digest.Sum(nil))
}

func decodeEjianbaoValue(t *testing.T, value any) map[string]any {
	t.Helper()
	encoded, err := common.Marshal(value)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, common.Unmarshal(encoded, &decoded))
	return decoded
}

func TestEjianbaoPluginContract(t *testing.T) {
	source, err := builtinplugins.Source("ejianbao")
	require.NoError(t, err)
	registry := jsplugin.NewRegistry()
	plugin, err := registry.RegisterFactory(source, jsplugin.Options{Key: "ejianbao"})
	require.NoError(t, err)
	assert.Equal(t, []string{"ejianbao-digitalhuman"}, plugin.Meta.Models)
	assert.Equal(t, "1.1.0", plugin.Meta.Version)
	assert.Equal(t, "per_task", plugin.Meta.FetchMode)
	assert.Contains(t, plugin.Meta.UsageSchema, "seconds")

	const key = "platform-secret"
	const envelope = `{"avatar_id":"avatar_1","voice_id":"voice_1","request_id":"request_1","script_text":"这是一段用于数字人口播的测试文案。","seconds":20,"rate":10}`
	requestBody := map[string]any{"model": "ejianbao-digitalhuman", "envelope": envelope, "signature": ejianbaoSignature(envelope, key)}
	context := map[string]any{
		"apiKey":      key,
		"baseUrl":     "https://saas.inferflow.dev/openapi/v1",
		"requestBody": requestBody,
	}

	t.Run("builds authenticated idempotent submission", func(t *testing.T) {
		value, callErr := plugin.Engine.Call(t.Context(), "buildSubmitRequest", context)
		require.NoError(t, callErr)
		descriptor := decodeEjianbaoValue(t, value)
		assert.Equal(t, "https://saas.inferflow.dev/openapi/v1/skills/digital_human_standard/runs", descriptor["url"])
		headers := descriptor["headers"].(map[string]any)
		assert.Equal(t, key, headers["X-API-Key"])
		assert.Equal(t, "request_1", headers["Idempotency-Key"])
		body := descriptor["body"].(map[string]any)["inputs"].(map[string]any)
		assert.Equal(t, "avatar_1", body["avatar_id"])
		assert.Equal(t, "voice_1", body["voice_id"])
		assert.Equal(t, "fast_segments", body["segmentation_mode"])
	})

	t.Run("requires the declared model", func(t *testing.T) {
		invalid := map[string]any{"apiKey": key, "requestBody": map[string]any{
			"model": "another-model", "envelope": envelope, "signature": ejianbaoSignature(envelope, key),
		}}
		_, callErr := plugin.Engine.Call(t.Context(), "extractUsage", invalid)
		require.ErrorContains(t, callErr, "unsupported digital-human model")
	})

	t.Run("meters signed estimate and actual output", func(t *testing.T) {
		value, callErr := plugin.Engine.Call(t.Context(), "extractUsage", map[string]any{
			"apiKey": key, "requestBody": requestBody, "usagePurpose": "facts",
		})
		require.NoError(t, callErr)
		assert.Equal(t, map[string]any{"seconds": float64(20)}, decodeEjianbaoValue(t, value))

		value, callErr = plugin.Engine.Call(t.Context(), "extractUsageOnComplete", nil, map[string]any{"status": "SUCCESS"}, map[string]any{
			"outputs": []any{map[string]any{"name": "video", "duration_seconds": 13.2, "download_url": "https://storage.example/video.mp4"}},
		})
		require.NoError(t, callErr)
		assert.Equal(t, map[string]any{"seconds": float64(14)}, decodeEjianbaoValue(t, value))
	})

	t.Run("maps submit and polling responses", func(t *testing.T) {
		value, callErr := plugin.Engine.Call(t.Context(), "buildQueryRequest", map[string]any{
			"apiKey": key, "baseUrl": "https://saas.inferflow.dev/openapi/v1", "taskId": "run_1",
		})
		require.NoError(t, callErr)
		assert.Equal(t, "https://saas.inferflow.dev/openapi/v1/runs/run_1", decodeEjianbaoValue(t, value)["url"])

		value, callErr = plugin.Engine.Call(t.Context(), "buildQueryRequest", map[string]any{
			"apiKey": key, "baseUrl": "https://saas.inferflow.dev/openapi/v1", "taskId": "run_1",
			"requestBody": map[string]any{"data": map[string]any{"status": "completed"}},
		})
		require.NoError(t, callErr)
		assert.Equal(t, "https://saas.inferflow.dev/openapi/v1/runs/run_1/outputs", decodeEjianbaoValue(t, value)["url"])

		value, callErr = plugin.Engine.Call(t.Context(), "parseSubmitResponse", nil, map[string]any{
			"statusCode": int64(200), "body": map[string]any{"run_id": "run_1"},
		})
		require.NoError(t, callErr)
		assert.Equal(t, "run_1", decodeEjianbaoValue(t, value)["taskId"])

		value, callErr = plugin.Engine.Call(t.Context(), "parseTaskResult", nil, map[string]any{"status": "running", "progress_percent": 42})
		require.NoError(t, callErr)
		progress := decodeEjianbaoValue(t, value)
		assert.Equal(t, "IN_PROGRESS", progress["status"])
		assert.Equal(t, "42%", progress["progress"])

		value, callErr = plugin.Engine.Call(t.Context(), "parseTaskResult", nil, map[string]any{"status": "partial_success", "progress_percent": 100})
		require.NoError(t, callErr)
		progress = decodeEjianbaoValue(t, value)
		assert.Equal(t, "IN_PROGRESS", progress["status"])
		assert.Equal(t, "99%", progress["progress"])

		value, callErr = plugin.Engine.Call(t.Context(), "parseTaskResult", nil, map[string]any{"items": []any{}})
		require.NoError(t, callErr)
		progress = decodeEjianbaoValue(t, value)
		assert.Equal(t, "IN_PROGRESS", progress["status"])
		assert.Equal(t, "99%", progress["progress"])

		value, callErr = plugin.Engine.Call(t.Context(), "parseTaskResult", nil, map[string]any{
			"items": []any{map[string]any{"type": "file", "name": "video", "download_url": "https://storage.example/video.mp4", "duration_seconds": 13.2}},
		})
		require.NoError(t, callErr)
		assert.Equal(t, "SUCCESS", decodeEjianbaoValue(t, value)["status"])

		value, callErr = plugin.Engine.Call(t.Context(), "parseTaskResult", nil, map[string]any{
			"outputs": []any{map[string]any{"name": "video", "url": "https://storage.example/video.mp4"}},
		})
		require.NoError(t, callErr)
		assert.Equal(t, "SUCCESS", decodeEjianbaoValue(t, value)["status"])

		value, callErr = plugin.Engine.Call(t.Context(), "parseTaskResult", nil, map[string]any{
			"items":   []any{},
			"outputs": []any{map[string]any{"name": "video", "url": "https://storage.example/video.mp4"}},
		})
		require.NoError(t, callErr)
		assert.Equal(t, "SUCCESS", decodeEjianbaoValue(t, value)["status"])

		value, callErr = plugin.Engine.Call(t.Context(), "parseTaskResult", nil, map[string]any{
			"items":   []any{map[string]any{"name": "manifest", "url": "https://storage.example/manifest.json"}},
			"outputs": []any{map[string]any{"name": "video", "url": "https://storage.example/video.mp4"}},
		})
		require.NoError(t, callErr)
		assert.Equal(t, "SUCCESS", decodeEjianbaoValue(t, value)["status"])

		value, callErr = plugin.Engine.Call(t.Context(), "parseTaskResult", nil, map[string]any{
			"status": "completed",
			"items":  []any{map[string]any{"name": "manifest", "url": "https://storage.example/manifest.json"}},
		})
		require.NoError(t, callErr)
		assert.Equal(t, "IN_PROGRESS", decodeEjianbaoValue(t, value)["status"])

		value, callErr = plugin.Engine.Call(t.Context(), "parseTaskResult", nil, map[string]any{
			"items": []any{map[string]any{"name": "manifest", "url": "https://storage.example/manifest.json"}},
		})
		require.NoError(t, callErr)
		assert.Equal(t, "IN_PROGRESS", decodeEjianbaoValue(t, value)["status"])

		value, callErr = plugin.Engine.Call(t.Context(), "parseTaskResult", nil, map[string]any{
			"status": "failed",
			"items":  []any{map[string]any{"name": "manifest", "url": "https://storage.example/manifest.json"}},
		})
		require.NoError(t, callErr)
		assert.Equal(t, "FAILURE", decodeEjianbaoValue(t, value)["status"])
	})

	t.Run("rejects forged envelopes and unsafe artifact ids", func(t *testing.T) {
		forged := map[string]any{"apiKey": key, "requestBody": map[string]any{"envelope": envelope, "signature": "0" + requestBody["signature"].(string)[1:]}}
		_, callErr := plugin.Engine.Call(t.Context(), "extractUsage", forged)
		require.ErrorContains(t, callErr, "account authorization required")

		_, callErr = plugin.Engine.Call(t.Context(), "buildQueryRequest", map[string]any{
			"apiKey": key, "baseUrl": "https://saas.inferflow.dev/openapi/v1", "taskId": "../private",
		})
		require.ErrorContains(t, callErr, "invalid identifier")
	})

	t.Run("projects a video artifact only after success", func(t *testing.T) {
		value, callErr := plugin.Engine.Call(t.Context(), "listArtifacts", map[string]any{
			"status": "SUCCESS",
			"data":   map[string]any{"items": []any{map[string]any{"name": "video", "download_url": "https://storage.example/video.mp4"}}},
		})
		require.NoError(t, callErr)
		encoded, marshalErr := common.Marshal(value)
		require.NoError(t, marshalErr)
		assert.JSONEq(t, `[{"key":"video","type":"video","mimeType":"video/mp4"}]`, string(encoded))

		value, callErr = plugin.Engine.Call(t.Context(), "buildContentRequest", map[string]any{
			"apiKey": key, "baseUrl": "https://saas.inferflow.dev/openapi/v1", "artifactKey": "video", "upstreamTaskId": "run_1",
			"data": map[string]any{"items": []any{map[string]any{"type": "file", "name": "video", "download_url": "https://storage.example/video.mp4"}}},
		})
		require.NoError(t, callErr)
		descriptor := decodeEjianbaoValue(t, value)
		assert.Equal(t, "https://storage.example/video.mp4", descriptor["url"])
		assert.Equal(t, true, descriptor["credentialless"])
	})
}
