package multimodal

import "context"

// AnalysisResult represents the result of a multimodal analysis.
type AnalysisResult struct {
	CameraID          string             `json:"camera_id"`
	Timestamp         int64              `json:"timestamp"`
	Analysis          string             `json:"analysis"`                     // Natural language analysis from the model
	Labels            []string           `json:"labels"`                       // Detected objects/activities
	Confidence        float32            `json:"confidence"`                   // Overall confidence score
	ImageURL          string             `json:"image_url,omitempty"`          // Data URL or public URL for the analyzed image
	TriggerDetections []TriggerDetection `json:"trigger_detections,omitempty"` // YOLO detections that triggered this analysis
	Metadata          map[string]string  `json:"metadata"`                     // Additional model-specific data
}

// TriggerDetection is the object detection context that caused an LLM analysis.
type TriggerDetection struct {
	Label      string     `json:"label"`
	Confidence float32    `json:"confidence"`
	Box        [4]float32 `json:"box"`
}

// Analyzer performs multimodal analysis on video frames.
// Implementations must be safe for concurrent use.
type Analyzer interface {
	// Analyze sends a frame to the multimodal model and returns analysis results.
	Analyze(ctx context.Context, frame []byte, prompt string) (*AnalysisResult, error)

	// IsAvailable checks if the analyzer is ready to process requests.
	IsAvailable() bool

	// Close cleans up resources.
	Close() error
}

// ProviderConfig holds common configuration for multimodal providers.
type ProviderConfig struct {
	Provider    string  `yaml:"provider" json:"provider"` // "deepseek", "openai", "qwen", etc.
	APIKey      string  `yaml:"api_key" json:"apiKey"`
	Endpoint    string  `yaml:"endpoint" json:"endpoint"`        // Custom API endpoint
	Model       string  `yaml:"model" json:"model"`              // Model name
	VisionModel string  `yaml:"vision_model" json:"visionModel"` // Vision-specific model
	MaxTokens   int     `yaml:"max_tokens" json:"maxTokens"`     // Max response tokens
	Temperature float32 `yaml:"temperature" json:"temperature"`  // Response creativity (0-1)
	Timeout     int     `yaml:"timeout" json:"timeout"`          // Request timeout in seconds
}

// DefaultPrompt is the default analysis prompt for surveillance footage.
const DefaultPrompt = `你是一个专业的监控画面分析师。请仔细观察这张监控画面截图，并提供以下分析：

1. **场景描述**：描述画面中的主要场景和环境
2. **人员活动**：识别画面中的人物及其活动
3. **物体识别**：识别重要的物体、车辆等
4. **异常检测**：指出任何可能的异常或安全隐患
5. **安全建议**：根据画面内容提供安全建议

请用简洁专业的语言描述，并在最后给出一个总体的安全评估等级（正常/注意/警告/危险）。`
