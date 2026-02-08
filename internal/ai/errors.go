package ai

import "fmt"

type APIError struct {
	Provider   string
	StatusCode int
	Body       string
}

func (e APIError) Error() string {
	if e.Provider == "" {
		return fmt.Sprintf("api error: status=%d body=%s", e.StatusCode, e.Body)
	}
	return fmt.Sprintf("%s api error: status=%d body=%s", e.Provider, e.StatusCode, e.Body)
}

func IsRetryableAPIError(err error) bool {
	if err == nil {
		return false
	}
	apiErr, ok := err.(APIError)
	if ok {
		// 408: timeout, 429: rate limit, 402: payment required
		return apiErr.StatusCode == 408 || apiErr.StatusCode == 429 || apiErr.StatusCode == 402 || apiErr.StatusCode >= 500
	}
	apiErrPtr, ok := err.(*APIError)
	if ok {
		return apiErrPtr.StatusCode == 408 || apiErrPtr.StatusCode == 429 || apiErrPtr.StatusCode == 402 || apiErrPtr.StatusCode >= 500
	}
	return false
}
