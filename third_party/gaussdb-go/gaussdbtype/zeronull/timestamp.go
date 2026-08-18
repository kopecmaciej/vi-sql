package zeronull

import (
	"database/sql/driver"
	"fmt"
	"time"

	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbtype"
)

type Timestamp time.Time

func (Timestamp) SkipUnderlyingTypePlan() {}

func (ts *Timestamp) ScanTimestamp(v gaussdbtype.Timestamp) error {
	if !v.Valid {
		*ts = Timestamp{}
		return nil
	}

	switch v.InfinityModifier {
	case gaussdbtype.Finite:
		*ts = Timestamp(v.Time)
		return nil
	case gaussdbtype.Infinity:
		return fmt.Errorf("cannot scan Infinity into *time.Time")
	case gaussdbtype.NegativeInfinity:
		return fmt.Errorf("cannot scan -Infinity into *time.Time")
	default:
		return fmt.Errorf("invalid InfinityModifier: %v", v.InfinityModifier)
	}
}

func (ts Timestamp) TimestampValue() (gaussdbtype.Timestamp, error) {
	if time.Time(ts).IsZero() {
		return gaussdbtype.Timestamp{}, nil
	}

	return gaussdbtype.Timestamp{Time: time.Time(ts), Valid: true}, nil
}

// Scan implements the database/sql Scanner interface.
func (ts *Timestamp) Scan(src any) error {
	if src == nil {
		*ts = Timestamp{}
		return nil
	}

	var nullable gaussdbtype.Timestamp
	err := nullable.Scan(src)
	if err != nil {
		return err
	}

	*ts = Timestamp(nullable.Time)

	return nil
}

// Value implements the database/sql/driver Valuer interface.
func (ts Timestamp) Value() (driver.Value, error) {
	if time.Time(ts).IsZero() {
		return nil, nil
	}

	return time.Time(ts), nil
}
