package client

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrEmptyResponse = errors.New("EasyDNS API returned an empty response body")

// ErrDomainRegistrationDisabled guards the billable registration path. See
// ADR-0003: registration requires a deliberate provider-level opt-in.
var ErrDomainRegistrationDisabled = errors.New("domain registration is disabled; set enable_domain_registration on the provider to register domains")

// AmbiguousWriteError means a mutation may have reached EasyDNS, but the
// client could not determine its final result. Callers must reconcile through
// a read; replaying the write could create a duplicate or repeat a side effect.
type AmbiguousWriteError struct {
	Method string
	Cause  error
}

func (err *AmbiguousWriteError) Error() string {
	return fmt.Sprintf("EasyDNS %s outcome is ambiguous: %v", err.Method, err.Cause)
}

func (err *AmbiguousWriteError) Unwrap() error {
	return err.Cause
}

func IsAmbiguousWrite(err error) bool {
	var ambiguous *AmbiguousWriteError
	return errors.As(err, &ambiguous)
}

type APIError struct {
	HTTPStatus int
	Code       int64
	Message    string
}

func (err *APIError) Error() string {
	message := "EasyDNS API request failed"
	if err.HTTPStatus != 0 {
		message += fmt.Sprintf(" with HTTP status %d", err.HTTPStatus)
	}
	if err.Code != 0 && int64(err.HTTPStatus) != err.Code {
		message += fmt.Sprintf(" and API code %d", err.Code)
	}
	if err.Message != "" {
		message += ": " + err.Message
	}
	return message
}

type NotFoundError struct {
	Resource string
	ID       string
}

func (err *NotFoundError) Error() string {
	return fmt.Sprintf("%s %s not found", err.Resource, err.ID)
}

func IsNotFound(err error) bool {
	var notFound *NotFoundError
	if errors.As(err, &notFound) {
		return true
	}

	var apiError *APIError
	return errors.As(err, &apiError) && (apiError.HTTPStatus == 404 || apiError.Code == 404)
}

type ResponseTooLargeError struct {
	Limit int64
}

type ReconciliationTimeoutError struct {
	Operation string
	Timeout   time.Duration
	Cause     error
}

func (err *ReconciliationTimeoutError) Error() string {
	message := fmt.Sprintf("EasyDNS %s was not observable within %s", err.Operation, err.Timeout)
	if err.Cause != nil {
		message += fmt.Sprintf(" after an ambiguous write: %v", err.Cause)
	}
	return message
}

func (err *ReconciliationTimeoutError) Unwrap() error {
	return err.Cause
}

// DomainDeletionNotObservableError means EasyDNS accepted the deletion and
// then kept reporting the domain as on-system. Observed against the sandbox:
// DELETE /domain/{domain} answers 200 OK while getDomainInfo continues to
// return onsystem "Y" indefinitely.
type DomainDeletionNotObservableError struct {
	Domain  string
	Timeout time.Duration
}

func (err *DomainDeletionNotObservableError) Error() string {
	return fmt.Sprintf(
		"EasyDNS accepted the deletion of %s but still reported it as on the system after %s. "+
			"Terraform has not removed it from state, because doing so would abandon a domain that may still bill. "+
			"Check the domain in the EasyDNS dashboard: deletion may be queued or may require cancelling the service.",
		err.Domain, err.Timeout)
}

type DuplicateRecordCandidatesError struct {
	IDs []string
}

// DuplicateMailmapCandidatesError means a create was accepted but more than
// one new remote mailmap matches the requested value, so choosing an ID would
// make Terraform state nondeterministic.
type DuplicateMailmapCandidatesError struct {
	IDs []string
}

func (err *DuplicateMailmapCandidatesError) Error() string {
	return fmt.Sprintf("EasyDNS mailmap creation produced multiple matching new mailmaps with IDs %s", strings.Join(err.IDs, ", "))
}

func (err *DuplicateRecordCandidatesError) Error() string {
	return fmt.Sprintf("EasyDNS record creation produced multiple matching new records with IDs %s", strings.Join(err.IDs, ", "))
}

func (err *ResponseTooLargeError) Error() string {
	return fmt.Sprintf("EasyDNS API response exceeded the %d byte limit", err.Limit)
}
