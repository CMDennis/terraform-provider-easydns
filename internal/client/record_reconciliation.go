package client

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

func (c *Client) reconcileCreatedRecord(ctx context.Context, desired CreateRecordRequest, beforeIDs map[string]struct{}, preferredID string, writeErr error) (*Record, error) {
	deadline := c.clock.Now().Add(c.recordReconcileTimeout)
	for {
		records, err := c.GetRecords(ctx, desired.Domain)
		if err != nil {
			return nil, fmt.Errorf("reconcile created record: %w", err)
		}

		if preferredID != "" {
			for _, record := range records {
				if record.ID == preferredID && RecordsEquivalent(record, desired) {
					result := record
					return &result, nil
				}
			}
		} else {
			candidates := make([]Record, 0, 1)
			for _, record := range records {
				if _, existed := beforeIDs[record.ID]; !existed && RecordsEquivalent(record, desired) {
					candidates = append(candidates, record)
				}
			}
			if len(candidates) == 1 {
				return &candidates[0], nil
			}
			if len(candidates) > 1 {
				ids := make([]string, len(candidates))
				for index := range candidates {
					ids[index] = candidates[index].ID
				}
				sort.Slice(ids, func(left, right int) bool { return lessRecordID(ids[left], ids[right]) })
				return nil, &DuplicateRecordCandidatesError{IDs: ids}
			}
		}

		if err := c.waitForRecordPoll(ctx, deadline); err != nil {
			return nil, reconciliationError("record creation", c.recordReconcileTimeout, writeErr, err)
		}
	}
}

func (c *Client) reconcileUpdatedRecord(ctx context.Context, recordID string, desired CreateRecordRequest, writeErr error) (*Record, error) {
	deadline := c.clock.Now().Add(c.recordReconcileTimeout)
	for {
		record, err := c.GetRecord(ctx, desired.Domain, recordID)
		if err == nil && RecordsEquivalent(*record, desired) {
			return record, nil
		}
		if err != nil && !IsNotFound(err) {
			return nil, fmt.Errorf("reconcile updated record: %w", err)
		}
		if err := c.waitForRecordPoll(ctx, deadline); err != nil {
			return nil, reconciliationError("record update", c.recordReconcileTimeout, writeErr, err)
		}
	}
}

func (c *Client) reconcileDeletedRecord(ctx context.Context, domain, recordID string, writeErr error) error {
	deadline := c.clock.Now().Add(c.recordReconcileTimeout)
	for {
		_, err := c.GetRecord(ctx, domain, recordID)
		if IsNotFound(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reconcile deleted record: %w", err)
		}
		if err := c.waitForRecordPoll(ctx, deadline); err != nil {
			return reconciliationError("record deletion", c.recordReconcileTimeout, writeErr, err)
		}
	}
}

func (c *Client) waitForRecordPoll(ctx context.Context, deadline time.Time) error {
	now := c.clock.Now()
	if !now.Before(deadline) {
		return errReconciliationDeadline
	}
	delay := c.recordPollInterval
	if remaining := deadline.Sub(now); delay > remaining {
		delay = remaining
	}
	return c.waiter.Wait(ctx, delay)
}

var errReconciliationDeadline = errors.New("record reconciliation deadline exceeded")

func reconciliationError(operation string, timeout time.Duration, writeErr, waitErr error) error {
	if !errors.Is(waitErr, errReconciliationDeadline) {
		if writeErr != nil {
			return fmt.Errorf("EasyDNS %s reconciliation stopped after an ambiguous write (%v): %w", operation, writeErr, waitErr)
		}
		return waitErr
	}
	return &ReconciliationTimeoutError{Operation: operation, Timeout: timeout, Cause: writeErr}
}
