package types

import "net/http"

// APIError 统一的 API 错误响应结构
type APIError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// ErrorResponse 错误响应包装
type ErrorResponse struct {
	Error APIError `json:"error"`
}

// NewAPIError 创建 API 错误
func NewAPIError(errType, message, code string) ErrorResponse {
	return ErrorResponse{
		Error: APIError{
			Type:    errType,
			Message: message,
			Code:    code,
		},
	}
}

// ErrorCodeFromStatus 根据 HTTP 状态码返回错误代码
func ErrorCodeFromStatus(statusCode int) string {
	switch statusCode {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusTooManyRequests:
		return "rate_limited"
	case http.StatusRequestTimeout:
		return "timeout"
	case http.StatusServiceUnavailable:
		return "overloaded"
	default:
		return "internal_error"
	}
}

// ErrorTypeFromStatus 根据 HTTP 状态码返回错误类型
func ErrorTypeFromStatus(statusCode int) string {
	switch statusCode {
	case http.StatusBadRequest:
		return "invalid_request_error"
	case http.StatusUnauthorized, http.StatusForbidden:
		return "authentication_error"
	case http.StatusNotFound:
		return "not_found_error"
	case http.StatusTooManyRequests:
		return "rate_limit_error"
	case http.StatusServiceUnavailable:
		return "overloaded_error"
	default:
		return "api_error"
	}
}
