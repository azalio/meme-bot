package service

import (
    "bytes"
    "context"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "github.com/azalio/meme-bot/internal/otel/metrics"
    "github.com/azalio/meme-bot/pkg/logger"
    "go.opentelemetry.io/otel/attribute"
)

type CloudflareAIServiceImpl struct {
    logger    *logger.Logger
    workerURL string
}

func NewCloudflareAIService(log *logger.Logger) *CloudflareAIServiceImpl {
    return &CloudflareAIServiceImpl{
        logger:    log,
        workerURL: "https://snowy-sun-10f9.azalio.workers.dev/",
    }
}

func (s *CloudflareAIServiceImpl) GenerateImage(ctx context.Context, prompt string) ([]byte, error) {
    startTime := time.Now()
    defer func() {
        metrics.APIResponseTime.Observe(time.Since(startTime).Seconds(), 
            attribute.String("service", "cloudflare_ai"))
    }()

    requestBody, err := json.Marshal(map[string]interface{}{
        "prompt": prompt,
        "steps":  4,
    })
    if err != nil {
        s.logger.Error(ctx, "Failed to marshal request", map[string]interface{}{
            "error": err.Error(),
        })
        metrics.CloudflareAIFailureCounter.Inc("marshal_error")
        return nil, fmt.Errorf("marshalling request: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, "POST", s.workerURL, bytes.NewBuffer(requestBody))
    if err != nil {
        s.logger.Error(ctx, "Failed to create request", map[string]interface{}{
            "error": err.Error(),
        })
        metrics.CloudflareAIFailureCounter.Inc("request_error")
        return nil, fmt.Errorf("creating request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{Timeout: 30 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        s.logger.Error(ctx, "Request failed", map[string]interface{}{
            "error": err.Error(),
        })
        metrics.CloudflareAIFailureCounter.Inc("http_error")
        return nil, fmt.Errorf("making request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        s.logger.Error(ctx, "Unexpected status code", map[string]interface{}{
            "status_code": resp.StatusCode,
        })
        metrics.CloudflareAIFailureCounter.Inc("status_error")
        return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
    }

    var result struct {
        Image string `json:"image"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        s.logger.Error(ctx, "Failed to decode response", map[string]interface{}{
            "error": err.Error(),
        })
        metrics.CloudflareAIFailureCounter.Inc("decode_error")
        return nil, fmt.Errorf("decoding response: %w", err)
    }

    imageData, err := base64.StdEncoding.DecodeString(result.Image)
    if err != nil {
        s.logger.Error(ctx, "Failed to decode image", map[string]interface{}{
            "error": err.Error(),
        })
        metrics.CloudflareAIFailureCounter.Inc("image_decode_error")
        return nil, fmt.Errorf("decoding image: %w", err)
    }

    metrics.CloudflareAISuccessCounter.Inc("success")
    return imageData, nil
}
