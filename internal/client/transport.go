package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type requestOptions struct {
	safeToRetry bool
	semantics   requestSemantics
}

type requestSemantics uint8

const (
	requestSemanticsFromMethod requestSemantics = iota
	requestSemanticsRead
	requestSemanticsMutation
)

type apiErrorEnvelope struct {
	Error *struct {
		Code    flexibleInt64 `json:"code"`
		Message string        `json:"message"`
	} `json:"error"`
	Status flexibleInt64 `json:"status"`
	Msg    string        `json:"msg"`
}

func (c *Client) doJSON(ctx context.Context, method string, endpoint *url.URL, body any, output any, options requestOptions) error {
	var requestBody []byte
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode EasyDNS request: %w", err)
		}
		requestBody = encoded
	}

	attempts := 1
	if options.safeToRetry {
		attempts = c.retryPolicy.MaxAttempts
	}

	for attempt := 1; attempt <= attempts; attempt++ {
		if err := c.reserveRequest(ctx); err != nil {
			return err
		}

		var reader io.Reader
		if requestBody != nil {
			reader = bytes.NewReader(requestBody)
		}

		request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), reader)
		if err != nil {
			return fmt.Errorf("create EasyDNS request: %w", err)
		}
		request.SetBasicAuth(c.token, c.key)
		request.Header.Set("Accept", "application/json")
		request.Header.Set("User-Agent", c.userAgent)
		if requestBody != nil {
			request.Header.Set("Content-Type", "application/json")
		}

		response, err := c.httpClient.Do(request)
		if err != nil {
			if ctx.Err() != nil {
				return markAmbiguousWrite(method, options, ctx.Err())
			}
			if options.safeToRetry && attempt < attempts {
				if err := c.waiter.Wait(ctx, c.retryDelay(attempt, "")); err != nil {
					return err
				}
				continue
			}
			return markAmbiguousWrite(method, options, fmt.Errorf("send EasyDNS request: %w", err))
		}

		responseBody, readErr := readLimited(response.Body, c.maxResponseBodyBytes)
		closeErr := response.Body.Close()
		if readErr != nil {
			return markAmbiguousWrite(method, options, readErr)
		}
		if closeErr != nil {
			return markAmbiguousWrite(method, options, fmt.Errorf("close EasyDNS response: %w", closeErr))
		}

		if options.safeToRetry && attempt < attempts && isRetryableStatus(response.StatusCode) {
			if err := c.waiter.Wait(ctx, c.retryDelay(attempt, response.Header.Get("Retry-After"))); err != nil {
				return err
			}
			continue
		}

		if response.StatusCode < 200 || response.StatusCode >= 300 {
			err := parseAPIError(response.StatusCode, responseBody)
			if isAmbiguousMutationStatus(response.StatusCode) {
				return markAmbiguousWrite(method, options, err)
			}
			return err
		}

		if err := errorFromSuccessEnvelope(response.StatusCode, responseBody); err != nil {
			return err
		}
		if output == nil {
			return nil
		}
		if len(bytes.TrimSpace(responseBody)) == 0 {
			return markAmbiguousWrite(method, options, ErrEmptyResponse)
		}
		if err := decodeJSON(responseBody, output); err != nil {
			return markAmbiguousWrite(method, options, fmt.Errorf("decode EasyDNS response: %w", err))
		}
		return nil
	}

	return errors.New("EasyDNS request exhausted retries")
}

func markAmbiguousWrite(method string, options requestOptions, err error) error {
	if options.semantics == requestSemanticsRead {
		return err
	}
	if options.semantics == requestSemanticsMutation {
		return &AmbiguousWriteError{Method: method, Cause: err}
	}
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return &AmbiguousWriteError{Method: method, Cause: err}
	default:
		return err
	}
}

func readLimited(reader io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read EasyDNS response: %w", err)
	}
	if int64(len(data)) > limit {
		return nil, &ResponseTooLargeError{Limit: limit}
	}
	return data, nil
}

func decodeJSON(data []byte, output any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	return decoder.Decode(output)
}

func errorFromSuccessEnvelope(httpStatus int, data []byte) error {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}

	var envelope apiErrorEnvelope
	if err := decodeJSON(data, &envelope); err != nil {
		return nil
	}
	if envelope.Error != nil {
		return &APIError{
			HTTPStatus: httpStatus,
			Code:       envelope.Error.Code.Value,
			Message:    sanitizeMessage(envelope.Error.Message),
		}
	}
	if envelope.Status.Set && envelope.Status.Value >= 400 {
		return &APIError{HTTPStatus: httpStatus, Code: envelope.Status.Value, Message: sanitizeMessage(envelope.Msg)}
	}
	return nil
}

func parseAPIError(httpStatus int, data []byte) error {
	apiError := &APIError{HTTPStatus: httpStatus}
	var envelope apiErrorEnvelope
	if len(bytes.TrimSpace(data)) > 0 && decodeJSON(data, &envelope) == nil {
		if envelope.Error != nil {
			apiError.Code = envelope.Error.Code.Value
			apiError.Message = sanitizeMessage(envelope.Error.Message)
		} else if envelope.Status.Set {
			apiError.Code = envelope.Status.Value
			apiError.Message = sanitizeMessage(envelope.Msg)
		}
	}
	return apiError
}

func sanitizeMessage(message string) string {
	message = strings.Map(func(character rune) rune {
		if character < 0x20 && character != '\t' && character != '\n' {
			return -1
		}
		return character
	}, strings.TrimSpace(message))
	const maximumLength = 512
	if len(message) > maximumLength {
		message = message[:maximumLength] + "..."
	}
	return message
}

func isRetryableStatus(status int) bool {
	switch status {
	case 420, http.StatusTooManyRequests, http.StatusInternalServerError,
		http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func isAmbiguousMutationStatus(status int) bool {
	switch status {
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func (c *Client) retryDelay(attempt int, retryAfter string) time.Duration {
	if parsed, ok := parseRetryAfter(retryAfter, c.clock.Now()); ok {
		if parsed > c.retryPolicy.MaxDelay {
			return c.retryPolicy.MaxDelay
		}
		return parsed
	}

	delay := c.retryPolicy.InitialDelay
	for index := 1; index < attempt; index++ {
		if delay >= c.retryPolicy.MaxDelay/2 {
			delay = c.retryPolicy.MaxDelay
			break
		}
		delay *= 2
	}
	if delay > c.retryPolicy.MaxDelay {
		delay = c.retryPolicy.MaxDelay
	}
	return c.retryPolicy.Jitter(delay)
}

func parseRetryAfter(value string, now time.Time) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second, true
	}
	if when, err := http.ParseTime(value); err == nil {
		delay := when.Sub(now)
		if delay < 0 {
			delay = 0
		}
		return delay, true
	}
	return 0, false
}

func defaultJitter(delay time.Duration) time.Duration {
	if delay <= 0 {
		return 0
	}
	factor := 0.8 + rand.Float64()*0.4
	return time.Duration(float64(delay) * factor)
}
