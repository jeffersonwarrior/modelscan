package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"sync"
	"time"
)

// WhisperProvider implements the Provider interface for OpenAI Whisper
type WhisperProvider struct {
	apiKey    string
	baseURL   string
	client    *http.Client
	endpoints []Endpoint
}

// NewWhisperProvider creates a new Whisper provider instance
func NewWhisperProvider(apiKey string) Provider {
	return &WhisperProvider{
		apiKey:  apiKey,
		baseURL: "https://api.openai.com/v1",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func init() {
	RegisterProvider("whisper", NewWhisperProvider)
}

// whisperModelResponse represents the response from /models endpoint
type whisperModelResponse struct {
	Data []whisperModel `json:"data"`
}

type whisperModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// whisperTranscriptionResponse represents the response from /audio/transcriptions endpoint
type whisperTranscriptionResponse struct {
	Text string `json:"text"`
}

func (p *WhisperProvider) ValidateEndpoints(ctx context.Context, verbose bool) error {
	endpoints := p.GetEndpoints()

	// Parallelize endpoint testing for better performance
	var wg sync.WaitGroup
	var mu sync.Mutex // Protect concurrent writes to endpoint status

	for i := range endpoints {
		wg.Add(1)
		go func(endpoint *Endpoint) {
			defer wg.Done()

			if verbose {
				mu.Lock()
				fmt.Printf("  Testing endpoint: %s %s\n", endpoint.Method, endpoint.Path)
				mu.Unlock()
			}

			start := time.Now()
			err := p.testEndpoint(ctx, endpoint)
			latency := time.Since(start)

			mu.Lock()
			endpoint.Latency = latency
			if err != nil {
				endpoint.Status = StatusFailed
				endpoint.Error = err.Error()
				if verbose {
					fmt.Printf("    ✗ Failed: %v\n", err)
				}
			} else {
				endpoint.Status = StatusWorking
				if verbose {
					fmt.Printf("    ✓ Working (latency: %v)\n", latency)
				}
			}
			mu.Unlock()
		}(&endpoints[i])
	}

	wg.Wait()
	p.endpoints = endpoints
	return nil
}

func (p *WhisperProvider) testEndpoint(ctx context.Context, endpoint *Endpoint) error {
	url := p.baseURL + endpoint.Path

	req, err := http.NewRequestWithContext(ctx, endpoint.Method, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	// Add headers
	for k, v := range endpoint.Headers {
		req.Header.Set(k, v)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return nil
}

func (p *WhisperProvider) ListModels(ctx context.Context, verbose bool) ([]Model, error) {
	if verbose {
		fmt.Println("Fetching Whisper models from OpenAI API...")
	}

	url := p.baseURL + "/models"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var modelsResp whisperModelResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	var models []Model
	for _, m := range modelsResp.Data {
		// Only include whisper models
		if m.ID != "whisper-1" {
			continue
		}

		model := Model{
			ID:             m.ID,
			Name:           "Whisper",
			Description:    "General-purpose speech recognition model",
			CostPer1MIn:    6000.0, // $0.006 per minute = $6 per 1000 minutes
			CostPer1MOut:   0,      // No output cost
			ContextWindow:  0,      // Not applicable for audio
			MaxTokens:      0,      // Not applicable for audio
			SupportsImages: false,
			SupportsTools:  false,
			CanReason:      false,
			CanStream:      false,
			CreatedAt:      time.Unix(m.Created, 0).Format(time.RFC3339),
			Categories:     []string{"audio", "transcription", "stt"},
			Capabilities: map[string]string{
				"audio_formats": "mp3,mp4,mpeg,mpga,m4a,wav,webm",
				"max_file_size": "25MB",
				"languages":     "multilingual",
			},
		}

		models = append(models, model)

		if verbose {
			fmt.Printf("  Found model: %s (%s)\n", model.ID, model.Name)
		}
	}

	return models, nil
}

func (p *WhisperProvider) GetCapabilities() ProviderCapabilities {
	return ProviderCapabilities{
		SupportsChat:         false,
		SupportsFIM:          false,
		SupportsEmbeddings:   false,
		SupportsFineTuning:   false,
		SupportsAgents:       false,
		SupportsFileUpload:   true, // Audio file upload required
		SupportsStreaming:    false,
		SupportsJSONMode:     true, // Can return JSON format
		SupportsVision:       false,
		SupportsAudio:        true, // Primary capability
		SupportedParameters:  []string{"file", "model", "language", "prompt", "response_format", "temperature"},
		SecurityFeatures:     []string{"SOC2", "GDPR"},
		MaxRequestsPerMinute: 50,
		MaxTokensPerRequest:  0, // Not applicable for audio
	}
}

func (p *WhisperProvider) GetEndpoints() []Endpoint {
	return []Endpoint{
		{
			Path:        "/models",
			Method:      "GET",
			Description: "List available models",
			Headers: map[string]string{
				"Authorization": "Bearer " + p.apiKey,
			},
			Status: StatusUnknown,
		},
		{
			Path:        "/audio/transcriptions",
			Method:      "POST",
			Description: "Transcribe audio to text",
			Headers: map[string]string{
				"Authorization": "Bearer " + p.apiKey,
			},
			Status: StatusUnknown,
		},
		{
			Path:        "/audio/translations",
			Method:      "POST",
			Description: "Translate audio to English",
			Headers: map[string]string{
				"Authorization": "Bearer " + p.apiKey,
			},
			Status: StatusUnknown,
		},
	}
}

func (p *WhisperProvider) TestModel(ctx context.Context, modelID string, verbose bool) error {
	if verbose {
		fmt.Printf("Testing Whisper model: %s\n", modelID)
	}

	// Create a minimal WAV file (44-byte header + 1 sample)
	wavData := createMinimalWAV()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add file field
	fileWriter, err := writer.CreateFormFile("file", "test.wav")
	if err != nil {
		return fmt.Errorf("create form file: %w", err)
	}
	if _, err := fileWriter.Write(wavData); err != nil {
		return fmt.Errorf("write file data: %w", err)
	}

	// Add model field
	if err := writer.WriteField("model", modelID); err != nil {
		return fmt.Errorf("write model field: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("close writer: %w", err)
	}

	url := p.baseURL + "/audio/transcriptions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, &buf)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	if verbose {
		fmt.Printf("  ✓ Model %s is working\n", modelID)
	}

	return nil
}

// createMinimalWAV creates a minimal valid WAV file for testing
func createMinimalWAV() []byte {
	// WAV header (44 bytes) + minimal audio data
	wav := make([]byte, 0, 100)

	// RIFF header
	wav = append(wav, []byte("RIFF")...)
	wav = appendUint32LE(wav, 36) // ChunkSize (36 + data size)
	wav = append(wav, []byte("WAVE")...)

	// fmt subchunk
	wav = append(wav, []byte("fmt ")...)
	wav = appendUint32LE(wav, 16)    // Subchunk1Size
	wav = appendUint16LE(wav, 1)     // AudioFormat (PCM)
	wav = appendUint16LE(wav, 1)     // NumChannels (mono)
	wav = appendUint32LE(wav, 8000)  // SampleRate
	wav = appendUint32LE(wav, 16000) // ByteRate
	wav = appendUint16LE(wav, 2)     // BlockAlign
	wav = appendUint16LE(wav, 16)    // BitsPerSample

	// data subchunk
	wav = append(wav, []byte("data")...)
	wav = appendUint32LE(wav, 0) // Subchunk2Size (0 for empty audio)

	return wav
}

func appendUint32LE(b []byte, v uint32) []byte {
	return append(b, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}

func appendUint16LE(b []byte, v uint16) []byte {
	return append(b, byte(v), byte(v>>8))
}
