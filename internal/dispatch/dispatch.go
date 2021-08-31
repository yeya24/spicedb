package dispatch

import (
	"context"
	"errors"
	"fmt"

	v1 "github.com/authzed/spicedb/internal/proto/dispatch/v1"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// ErrMaxDepth is returned from CheckDepth when the max depth is exceeded.
var ErrMaxDepth = errors.New("max depth exceeded")

// Dispatcher interface describes a method for passing subchecks off to additional machines.
type Dispatcher interface {
	Check
	Expand
	Lookup
}

// Check interface describes just the methods required to dispatch check requests.
type Check interface {
	// DispatchCheck submits a single check request and returns its result.
	DispatchCheck(ctx context.Context, req *v1.DispatchCheckRequest) (*v1.DispatchCheckResponse, error)
}

// Expand interface describes just the methods required to dispatch expand requests.
type Expand interface {
	// DispatchExpand submits a single expand request and returns its result.
	DispatchExpand(ctx context.Context, req *v1.DispatchExpandRequest) (*v1.DispatchExpandResponse, error)
}

// Lookup interface describes just the methods required to dispatch lookup requests.
type Lookup interface {
	// DispatchLookup submits a single lookup request and returns its result.
	DispatchLookup(ctx context.Context, req *v1.DispatchLookupRequest) (*v1.DispatchLookupResponse, error)
}

// HasMetadata is an interface for requests containing resolver metadata.
type HasMetadata interface {
	zerolog.LogObjectMarshaler

	GetMetadata() *v1.ResolverMeta
}

// CheckDepth returns ErrMaxDepth if there is insufficient depth remaining to dispatch.
func CheckDepth(req HasMetadata) error {
	metadata := req.GetMetadata()
	if metadata == nil {
		log.Warn().Object("req", req).Msg("request missing metadata")
		return fmt.Errorf("request missing metadata")
	}

	if metadata.DepthRemaining == 0 {
		return ErrMaxDepth
	}

	return nil
}
