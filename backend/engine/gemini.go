package engine

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/genai"
)

// loadEnv는 실행 위치 근처에서 .env 파일을 탐색하여 환경변수를 로드합니다.
func loadEnv() {
	execDir, err := os.Getwd()
	if err != nil {
		return
	}
	paths := []string{
		filepath.Join(execDir, ".env"),
		filepath.Join(execDir, "poc", "moss", "backend", ".env"),
	}

	for _, p := range paths {
		file, err := os.Open(p)
		if err != nil {
			continue
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				val = strings.Trim(val, `"'`)
				os.Setenv(key, val)
			}
		}
		break
	}
}

// CallGeminiAPI는 Google Cloud Vertex AI API를 호출하여 자연어 생성 결과를 반환합니다.
func CallGeminiAPI(ctx context.Context, prompt string, jsonFormat bool) (string, error) {
	loadEnv()

	projectID := os.Getenv("GCP_PROJECT_ID")
	if projectID == "" {
		projectID = "gai-llm-poc"
	}

	location := os.Getenv("GCP_LOCATION")
	if location == "" {
		location = "us"
	}

	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = "gemini-3.5-flash"
	}

	// 구글 GenAI SDK를 이용한 Vertex AI 클라이언트 초기화 (파이썬 예제의 vertexai=True에 대응)
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend:  genai.BackendVertexAI,
		Project:  projectID,
		Location: location,
	})
	if err != nil {
		return "", fmt.Errorf("Vertex AI 클라이언트 초기화 실패: %w", err)
	}

	config := &genai.GenerateContentConfig{}
	if jsonFormat {
		config.ResponseMIMEType = "application/json"
	}

	resp, err := client.Models.GenerateContent(ctx, model, genai.Text(prompt), config)
	if err != nil {
		log.Printf("[GEMINI ERROR] GenerateContent API 실패 (모델: %s, 프로젝트: %s, 리전: %s): %v", model, projectID, location, err)
		return "", fmt.Errorf("Vertex AI 콘텐츠 생성 실패: %w", err)
	}

	// 응답 결과에서 텍스트 추출 (genai SDK의 헬퍼 메서드 사용)
	text := resp.Text()
	if text == "" {
		return "", fmt.Errorf("Vertex AI 응답이 비어 있거나 텍스트를 추출할 수 없습니다")
	}

	return text, nil
}
