package errors

import (
	"errors"
	"testing"

	flyteAdminError "github.com/flyteorg/flyteadmin/pkg/errors"
	mockScope "github.com/flyteorg/flytestdlib/promutils"

	"github.com/jackc/pgconn"
	"github.com/magiconair/properties/assert"
	"google.golang.org/grpc/codes"
)

func TestToFlyteAdminError_InvalidPqError(t *testing.T) {
	err := errors.New("foo")
	transformedErr := NewPostgresErrorTransformer(mockScope.NewTestScope()).ToFlyteAdminError(err)
	assert.Equal(t, codes.Internal, transformedErr.(flyteAdminError.FlyteAdminError).Code())
	assert.Equal(t, "unexpected error type for: foo", transformedErr.(flyteAdminError.FlyteAdminError).Error())
}

func TestToFlyteAdminError_UniqueConstraintViolation(t *testing.T) {
	err := &pgconn.PgError{
		Code:    "23505",
		Message: "message",
	}
	transformedErr := NewPostgresErrorTransformer(mockScope.NewTestScope()).ToFlyteAdminError(err)
	assert.Equal(t, codes.AlreadyExists, transformedErr.(flyteAdminError.FlyteAdminError).Code())
	assert.Equal(t, "value with matching already exists (message)",
		transformedErr.(flyteAdminError.FlyteAdminError).Error())
}

func TestToFlyteAdminError_UnrecognizedPostgresError(t *testing.T) {
	err := &pgconn.PgError{
		Code:    "foo",
		Message: "message",
	}
	transformedErr := NewPostgresErrorTransformer(mockScope.NewTestScope()).ToFlyteAdminError(err)
	assert.Equal(t, codes.Unknown, transformedErr.(flyteAdminError.FlyteAdminError).Code())
	assert.Equal(t, "failed database operation with message",
		transformedErr.(flyteAdminError.FlyteAdminError).Error())
}
