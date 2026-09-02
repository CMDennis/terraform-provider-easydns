package client

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestClientErrorTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "HTTP only", err: &APIError{HTTPStatus: 403}, want: "HTTP status 403"},
		{name: "API code", err: &APIError{HTTPStatus: 200, Code: 123, Message: "bad"}, want: "API code 123: bad"},
		{name: "matching status and code", err: &APIError{HTTPStatus: 420, Code: 420}, want: "HTTP status 420"},
		{name: "not found", err: &NotFoundError{Resource: "record", ID: "123"}, want: "record 123 not found"},
		{name: "too large", err: &ResponseTooLargeError{Limit: 10}, want: "10 byte limit"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := test.err.Error(); !strings.Contains(got, test.want) {
				t.Fatalf("Error()=%q, want containing %q", got, test.want)
			}
		})
	}
}

func TestIsNotFound(t *testing.T) {
	t.Parallel()

	if !IsNotFound(&NotFoundError{Resource: "record", ID: "123"}) {
		t.Fatal("typed not-found was not recognized")
	}
	if !IsNotFound(&APIError{HTTPStatus: 404}) {
		t.Fatal("HTTP 404 was not recognized")
	}
	if !IsNotFound(&APIError{HTTPStatus: 200, Code: 404}) {
		t.Fatal("API 404 was not recognized")
	}
	if IsNotFound(errors.New("record not found")) {
		t.Fatal("message text was incorrectly treated as typed not-found")
	}
}

func TestAmbiguousWriteError(t *testing.T) {
	t.Parallel()

	cause := errors.New("connection reset")
	err := &AmbiguousWriteError{Method: "PUT", Cause: cause}
	if got := err.Error(); !strings.Contains(got, "PUT outcome is ambiguous") {
		t.Fatalf("Error()=%q", got)
	}
	if !IsAmbiguousWrite(err) {
		t.Fatal("ambiguous write was not recognized")
	}
	if !errors.Is(err, cause) {
		t.Fatal("ambiguous write did not preserve its cause")
	}
	if IsAmbiguousWrite(cause) {
		t.Fatal("ordinary error was classified as an ambiguous write")
	}
}

func TestReconciliationErrors(t *testing.T) {
	t.Parallel()

	cause := &AmbiguousWriteError{Method: "PUT", Cause: ErrEmptyResponse}
	timeout := &ReconciliationTimeoutError{Operation: "record creation", Timeout: 2 * time.Second, Cause: cause}
	if !strings.Contains(timeout.Error(), "not observable within 2s") || !errors.Is(timeout, ErrEmptyResponse) {
		t.Fatalf("timeout=%v", timeout)
	}
	duplicate := &DuplicateRecordCandidatesError{IDs: []string{"2", "10"}}
	if !strings.Contains(duplicate.Error(), "2, 10") {
		t.Fatalf("duplicate=%v", duplicate)
	}
}
