package util

import (
	"compress/gzip"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// NonRetryableHTTPError 表示不应重试的HTTP异常
type NonRetryableHTTPError struct {
	StatusCode int
	Message    string
}

func (e *NonRetryableHTTPError) Error() string {
	return e.Message
}

// HTTPUtil HTTP工具类
type HTTPUtil struct {
	client *http.Client
}

// NewHTTPUtil 创建HTTP工具实例
func NewHTTPUtil() *HTTPUtil {
	return NewHTTPUtilWithProxy("")
}

// NewHTTPUtilWithProxy 创建带代理的HTTP工具实例
func NewHTTPUtilWithProxy(proxyURL string) *HTTPUtil {
	tr := &http.Transport{
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
		MaxConnsPerHost:       1024,
		MaxIdleConnsPerHost:   1024,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// 设置代理
	if proxyURL != "" {
		if proxy, err := url.Parse(proxyURL); err == nil {
			tr.Proxy = http.ProxyURL(proxy)
		}
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   100 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// 不自动跳转，手动处理以保留headers
			return http.ErrUseLastResponse
		},
	}

	return &HTTPUtil{
		client: client,
	}
}

// Do 执行HTTP请求
func (h *HTTPUtil) Do(req *http.Request) (*http.Response, error) {
	return h.client.Do(req)
}

// doGet 执行GET请求
func (h *HTTPUtil) doGet(urlStr string, headers map[string]string) (*http.Response, error) {
	Logger.Debug(fmt.Sprintf("正在获取: %s", urlStr))

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}

	// 设置默认headers
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("Cache-Control", "no-cache")

	// 设置自定义headers
	if headers != nil {
		for key, value := range headers {
			req.Header.Set(key, value)
		}
	}

	Logger.Debug(fmt.Sprintf("请求头: %v", req.Header))

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}

	// 手动处理重定向
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		location := resp.Header.Get("Location")
		if location != "" {
			var redirectURL string
			if strings.HasPrefix(location, "http") {
				redirectURL = location
			} else {
				// 相对URL
				baseURL, _ := url.Parse(urlStr)
				redirectURLParsed, _ := url.Parse(location)
				redirectURL = baseURL.ResolveReference(redirectURLParsed).String()
			}

			if redirectURL != urlStr {
				Logger.Debug(fmt.Sprintf("重定向到: %s", redirectURL))
				resp.Body.Close()
				return h.doGet(redirectURL, headers)
			}
		}
	}

	// 检查不可重试的状态码
	if h.isNonRetryableStatusCode(resp.StatusCode) {
		Logger.Error(fmt.Sprintf("HTTP %d %s: 请求失败，不进行重试", resp.StatusCode, resp.Status))
		return nil, &NonRetryableHTTPError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("HTTP %d %s: Request failed with non-retryable status code", resp.StatusCode, resp.Status),
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d %s", resp.StatusCode, resp.Status)
	}

	return resp, nil
}

// GetBytes 获取字节数据
func (h *HTTPUtil) GetBytes(urlStr string, headers map[string]string) ([]byte, error) {
	if strings.HasPrefix(urlStr, "file:") {
		// 处理文件URL
		fileURL, err := url.Parse(urlStr)
		if err != nil {
			return nil, err
		}
		return readFile(fileURL.Path)
	}

	resp, err := h.doGet(urlStr, headers)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var reader io.Reader = resp.Body

	// 处理 gzip 压缩
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Encoding")), "gzip") {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("创建gzip reader失败: %v", err)
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	Logger.Debug(fmt.Sprintf("获取到 %d 字节数据", len(data)))
	return data, nil
}

// GetString 获取字符串源码
func (h *HTTPUtil) GetString(urlStr string, headers map[string]string) (string, error) {
	resp, err := h.doGet(urlStr, headers)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// 检查是否是MPEG-TS流
	if h.isMPEGTS(resp) {
		return "Live TS Stream detected", nil
	}

	var reader io.Reader = resp.Body

	// 处理 gzip 压缩
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Encoding")), "gzip") {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return "", fmt.Errorf("创建gzip reader失败: %v", err)
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	result := string(data)
	Logger.Debug(fmt.Sprintf("获取到内容: %s", result))
	return result, nil
}

// GetStringAndURL 获取字符串源码和最终URL
func (h *HTTPUtil) GetStringAndURL(urlStr string, headers map[string]string) (string, string, error) {
	resp, err := h.doGet(urlStr, headers)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	finalURL := resp.Request.URL.String()

	// 检查是否是MPEG-TS流
	if h.isMPEGTS(resp) {
		return "Live TS Stream detected", finalURL, nil
	}

	var reader io.Reader = resp.Body

	// 处理 gzip 压缩
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Encoding")), "gzip") {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return "", "", fmt.Errorf("创建gzip reader失败: %v", err)
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", "", err
	}

	result := string(data)
	Logger.Debug(fmt.Sprintf("获取到内容: %s", result))
	return result, finalURL, nil
}

// PostBytes 发送POST请求
func (h *HTTPUtil) PostBytes(urlStr string, postData []byte) (string, error) {
	req, err := http.NewRequest("POST", urlStr, strings.NewReader(string(postData)))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(postData)))

	resp, err := h.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

// isMPEGTS 检查是否是MPEG-TS流
func (h *HTTPUtil) isMPEGTS(resp *http.Response) bool {
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	return contentType == "video/ts" || contentType == "video/mp2t" || contentType == "video/mpeg"
}

// isNonRetryableStatusCode 检查是否为不可重试的状态码
func (h *HTTPUtil) isNonRetryableStatusCode(statusCode int) bool {
	switch statusCode {
	case 401, 403, 404, 429, 500, 502, 503:
		return true
	default:
		return false
	}
}

// 全局代理URL
var globalProxyURL string

// 默认的HTTP工具实例
var DefaultHTTPUtil = NewHTTPUtil()

// SetHTTPProxy 设置全局HTTP代理
func SetHTTPProxy(proxyURL string) {
	globalProxyURL = proxyURL
	// 重新创建默认HTTP工具实例
	DefaultHTTPUtil = NewHTTPUtilWithProxy(proxyURL)
}

// GetHTTPProxy 获取当前HTTP代理
func GetHTTPProxy() string {
	return globalProxyURL
}

// 便捷函数
func GetBytes(urlStr string, headers map[string]string) ([]byte, error) {
	return DefaultHTTPUtil.GetBytes(urlStr, headers)
}

func GetString(urlStr string, headers map[string]string) (string, error) {
	return DefaultHTTPUtil.GetString(urlStr, headers)
}

func GetStringAndURL(urlStr string, headers map[string]string) (string, string, error) {
	return DefaultHTTPUtil.GetStringAndURL(urlStr, headers)
}

func PostBytes(urlStr string, postData []byte) (string, error) {
	return DefaultHTTPUtil.PostBytes(urlStr, postData)
}

func Do(req *http.Request) (*http.Response, error) {
	return DefaultHTTPUtil.Do(req)
}
