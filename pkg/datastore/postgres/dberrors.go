package postgres

import (
	"errors"

	"github.com/lib/pq"
	"github.com/lib/pq/pqerror"
)


const (
	// UniqueViolationError is raised if the unique constraint is violated
	UniqueViolationError = pqerror.Code("23505") // 'unique_violation'
)

// IsErrorCode a specific postgres specific error as defined by
// https://www.postgresql.org/docs/18/errcodes-appendix.html
func IsErrorCode(err error, errcode pqerror.Code) bool {
	if pgerr, ok := errors.AsType[*pq.Error](err); ok {
		return pgerr.Code == errcode
	}
	return false
}
