// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/polarsource/polar-go/models/apierrors"
)

// Polling constants for eventual consistency handling.
const (
	pollMaxAttempts = 10
	pollInterval    = 500 * time.Millisecond
)

// isNotFound checks if an error is a Polar API 404 ResourceNotFound error.
func isNotFound(err error, target **apierrors.ResourceNotFound) bool {
	return errors.As(err, target)
}

// optionalStringValue safely converts a *string to types.String,
// returning types.StringNull() if the pointer is nil.
func optionalStringValue(s *string) types.String {
	if s == nil {
		return types.StringNull()
	}
	return types.StringValue(*s)
}

// optionalInt64Value safely converts a *int64 to types.Int64,
// returning types.Int64Null() if the pointer is nil.
func optionalInt64Value(i *int64) types.Int64 {
	if i == nil {
		return types.Int64Null()
	}
	return types.Int64Value(*i)
}

// derefBool safely dereferences a *bool, returning false if nil.
func derefBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

// pollForVisibility polls fetch until it returns a result accepted by the
// accept function. If accept is nil, any successful fetch is accepted.
// When accept returns false, the reason string is included in the final
// error message if polling exhausts all attempts.
// Retries on ResourceNotFound errors. Respects context cancellation.
func pollForVisibility[T any](ctx context.Context, resourceType string, id string, fetch func() (*T, error), accept func(*T) (bool, string)) (*T, error) {
	var lastRejectReason string
	for i := 0; i < pollMaxAttempts; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(pollInterval):
			}
		}
		result, err := fetch()
		if err != nil {
			var notFound *apierrors.ResourceNotFound
			if isNotFound(err, &notFound) {
				continue
			}
			return nil, err
		}
		if accept != nil {
			ok, reason := accept(result)
			if !ok {
				lastRejectReason = reason
				continue
			}
		}
		return result, nil
	}
	msg := fmt.Sprintf("%s %s not visible after polling", resourceType, id)
	if lastRejectReason != "" {
		msg += ": " + lastRejectReason
	}
	return nil, fmt.Errorf("%s", msg)
}
