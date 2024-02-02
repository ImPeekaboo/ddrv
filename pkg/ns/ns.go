package ns

import (
	"database/sql/driver"
	"fmt"
)

type NullString string

func (ns *NullString) Scan(value interface{}) error {
	if value == nil {
		*ns = ""
	} else if v, ok := value.([]byte); ok {
		*ns = NullString(v)
	} else if v, ok := value.(string); ok {
		*ns = NullString(v)
	} else {
		return fmt.Errorf("cannot convert %v of type %T to NullString", value, value)
	}
	return nil
}

func (ns *NullString) Value() (driver.Value, error) {
	if *ns == "" {
		return nil, nil
	}
	return string(*ns), nil
}
