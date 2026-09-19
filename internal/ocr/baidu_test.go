package ocr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestBaiduOCR_Recognize_Success(t *testing.T) {
	// Mock token server
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := baiduTokenResponse{
			AccessToken: "test_access_token",
			ExpiresIn:   2592000,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer tokenServer.Close()

	// Mock OCR server
	ocrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Query().Get("access_token") != "test_access_token" {
			t.Errorf("unexpected access token: %s", r.URL.Query().Get("access_token"))
		}
		// Parse form
		if err := r.ParseForm(); err != nil {
			t.Fatalf("failed to parse form: %v", err)
		}
		image := r.FormValue("image")
		if image == "" {
			t.Error("empty image in request")
		}
		// Return mock OCR result
		resp := baiduOCRResponse{
			WordsResultNum: 2,
			WordsResult: []baiduWordItem{
				{Words: "Hello"},
				{Words: "World"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ocrServer.Close()

	// Override URLs for testing
	origTokenURL := baiduTokenURL
	origOCRURL := baiduOCRURL
	baiduTokenURL = tokenServer.URL
	baiduOCRURL = ocrServer.URL
	defer func() {
		baiduTokenURL = origTokenURL
		baiduOCRURL = origOCRURL
	}()

	// Set environment variables
	os.Setenv("BAIDU_OCR_API_KEY", "test_api_key")
	os.Setenv("BAIDU_OCR_SECRET_KEY", "test_secret_key")
	defer os.Unsetenv("BAIDU_OCR_API_KEY")
	defer os.Unsetenv("BAIDU_OCR_SECRET_KEY")

	provider, err := NewBaiduOCR()
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	image := []byte("fake image data")
	text, err := provider.Recognize(context.Background(), image)
	if err != nil {
		t.Fatalf("Recognize failed: %v", err)
	}
	expected := "Hello\nWorld"
	if text != expected {
		t.Errorf("expected %q, got %q", expected, text)
	}
}

func TestBaiduOCR_Recognize_EmptyImage(t *testing.T) {
	os.Setenv("BAIDU_OCR_API_KEY", "test_api_key")
	os.Setenv("BAIDU_OCR_SECRET_KEY", "test_secret_key")
	defer os.Unsetenv("BAIDU_OCR_API_KEY")
	defer os.Unsetenv("BAIDU_OCR_SECRET_KEY")

	provider, err := NewBaiduOCR()
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	_, err = provider.Recognize(context.Background(), []byte{})
	if err == nil {
		t.Error("expected error for empty image")
	}
}

func TestBaiduOCR_Recognize_ImageTooLarge(t *testing.T) {
	os.Setenv("BAIDU_OCR_API_KEY", "test_api_key")
	os.Setenv("BAIDU_OCR_SECRET_KEY", "test_secret_key")
	defer os.Unsetenv("BAIDU_OCR_API_KEY")
	defer os.Unsetenv("BAIDU_OCR_SECRET_KEY")

	provider, err := NewBaiduOCR()
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	// Create image larger than maxImageSize
	largeImage := make([]byte, maxImageSize+1)
	_, err = provider.Recognize(context.Background(), largeImage)
	if err == nil {
		t.Error("expected error for too large image")
	}
}

func TestBaiduOCR_Recognize_TokenError(t *testing.T) {
	// Mock token server returning error
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer tokenServer.Close()

	origTokenURL := baiduTokenURL
	baiduTokenURL = tokenServer.URL
	defer func() {
		baiduTokenURL = origTokenURL
	}()

	os.Setenv("BAIDU_OCR_API_KEY", "test_api_key")
	os.Setenv("BAIDU_OCR_SECRET_KEY", "test_secret_key")
	defer os.Unsetenv("BAIDU_OCR_API_KEY")
	defer os.Unsetenv("BAIDU_OCR_SECRET_KEY")

	provider, err := NewBaiduOCR()
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	_, err = provider.Recognize(context.Background(), []byte("image"))
	if err == nil {
		t.Error("expected error for token request failure")
	}
}

func TestBaiduOCR_Recognize_OCRError(t *testing.T) {
	// Mock token server
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := baiduTokenResponse{
			AccessToken: "test_access_token",
			ExpiresIn:   2592000,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer tokenServer.Close()

	// Mock OCR server returning error
	ocrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := baiduOCRResponse{
			ErrorCode:    216201,
			ErrorMessage: "image format error",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ocrServer.Close()

	origTokenURL := baiduTokenURL
	origOCRURL := baiduOCRURL
	baiduTokenURL = tokenServer.URL
	baiduOCRURL = ocrServer.URL
	defer func() {
		baiduTokenURL = origTokenURL
		baiduOCRURL = origOCRURL
	}()

	os.Setenv("BAIDU_OCR_API_KEY", "test_api_key")
	os.Setenv("BAIDU_OCR_SECRET_KEY", "test_secret_key")
	defer os.Unsetenv("BAIDU_OCR_API_KEY")
	defer os.Unsetenv("BAIDU_OCR_SECRET_KEY")

	provider, err := NewBaiduOCR()
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	_, err = provider.Recognize(context.Background(), []byte("image"))
	if err == nil {
		t.Error("expected error for OCR failure")
	}
}

func TestBaiduOCR_Recognize_EmptyWordsResult(t *testing.T) {
	// Mock token server
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := baiduTokenResponse{
			AccessToken: "test_access_token",
			ExpiresIn:   2592000,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer tokenServer.Close()

	// Mock OCR server returning empty result
	ocrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := baiduOCRResponse{
			WordsResultNum: 0,
			WordsResult:    []baiduWordItem{},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer ocrServer.Close()

	origTokenURL := baiduTokenURL
	origOCRURL := baiduOCRURL
	baiduTokenURL = tokenServer.URL
	baiduOCRURL = ocrServer.URL
	defer func() {
		baiduTokenURL = origTokenURL
		baiduOCRURL = origOCRURL
	}()

	os.Setenv("BAIDU_OCR_API_KEY", "test_api_key")
	os.Setenv("BAIDU_OCR_SECRET_KEY", "test_secret_key")
	defer os.Unsetenv("BAIDU_OCR_API_KEY")
	defer os.Unsetenv("BAIDU_OCR_SECRET_KEY")

	provider, err := NewBaiduOCR()
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	_, err = provider.Recognize(context.Background(), []byte("image"))
	if err == nil {
		t.Error("expected error for empty words result")
	}
}

func TestBaiduOCR_NewBaiduOCR_MissingEnv(t *testing.T) {
	// Ensure env vars are not set
	os.Unsetenv("BAIDU_OCR_API_KEY")
	os.Unsetenv("BAIDU_OCR_SECRET_KEY")

	_, err := NewBaiduOCR()
	if err == nil {
		t.Error("expected error for missing environment variables")
	}
}
