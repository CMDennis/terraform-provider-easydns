package provider

import (
	"time"

	"github.com/CMDennis/terraform-provider-easydns/internal/client"
)

type Client = client.Client
type Record = client.Record
type CreateRecordRequest = client.CreateRecordRequest
type Zone = client.Zone
type RecordWriteMode = client.RecordWriteMode
type ParsedRecord = client.ParsedRecord
type ZoneSOA = client.ZoneSOA
type PageOptions = client.PageOptions
type GeoRegion = client.GeoRegion
type GeoRegionsResult = client.GeoRegionsResult
type Domain = client.Domain
type DomainSummary = client.DomainSummary
type DomainService = client.DomainService
type DomainCurrency = client.DomainCurrency
type Contact = client.Contact
type ContactSet = client.ContactSet
type CreateDomainRequest = client.CreateDomainRequest
type DomainCreation = client.DomainCreation
type GlueRecord = client.GlueRecord
type GlueRecordRequest = client.GlueRecordRequest
type RegistrationStatus = client.RegistrationStatus
type RegistrationSettingsRequest = client.RegistrationSettingsRequest
type RenewalAction = client.RenewalAction
type Mailmap = client.Mailmap
type MailmapRequest = client.MailmapRequest
type User = client.User
type ServiceDescription = client.ServiceDescription
type SubscriptionServiceDescription = client.SubscriptionServiceDescription
type DomainPricingRequest = client.DomainPricingRequest
type DomainPricing = client.DomainPricing
type PricedService = client.PricedService
type PrimaryNameserverResult = client.PrimaryNameserverResult

const (
	RecordWriteModeSynchronous  = client.RecordWriteModeSynchronous
	RecordWriteModeAsynchronous = client.RecordWriteModeAsynchronous
)

var IsNotFound = client.IsNotFound
var IsAmbiguousWrite = client.IsAmbiguousWrite
var NormalizeDomain = client.NormalizeDomain
var NormalizeHost = client.NormalizeHost
var NormalizeRecordType = client.NormalizeRecordType
var NormalizeRecordRdata = client.NormalizeRecordRdata
var RecordsEquivalent = client.RecordsEquivalent
var ParseRenewalAction = client.ParseRenewalAction
var ErrDomainRegistrationDisabled = client.ErrDomainRegistrationDisabled

func NewClient(baseURL, token, key string, useAsyncAPI bool) (*Client, error) {
	writeMode := client.RecordWriteModeSynchronous
	if useAsyncAPI {
		writeMode = client.RecordWriteModeAsynchronous
	}

	return NewClientWithMode(baseURL, token, key, writeMode)
}

func NewClientWithMode(baseURL, token, key string, writeMode RecordWriteMode) (*Client, error) {
	return NewConfiguredClient(baseURL, token, key, writeMode, false, 0, 0)
}

// NewConfiguredClient builds a client carrying every provider-level policy,
// including the ADR-0003 domain-registration opt-in.
// A zero pollInterval or reconcileTimeout keeps the client default.
func NewConfiguredClient(baseURL, token, key string, writeMode RecordWriteMode, enableDomainRegistration bool, pollInterval, reconcileTimeout time.Duration) (*Client, error) {
	return client.New(client.Config{
		BaseURL:                  baseURL,
		Token:                    token,
		Key:                      key,
		RecordWriteMode:          writeMode,
		EnableDomainRegistration: enableDomainRegistration,
		RecordPollInterval:       pollInterval,
		RecordReconcileTimeout:   reconcileTimeout,
	})
}
