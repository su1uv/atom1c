package database

import (
	"database/sql/driver"
	"fmt"
	"strings"

	"modernc.org/sqlite"
)

func init() {
	sqlite.MustRegisterDeterministicScalarFunction("unicode_lower", 1, func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		switch value := args[0].(type) {
		case string:
			return strings.ToLower(value), nil
		case []byte:
			return strings.ToLower(string(value)), nil
		case nil:
			return nil, nil
		default:
			return nil, fmt.Errorf("unicode_lower expects text, got %T", value)
		}
	})
}
