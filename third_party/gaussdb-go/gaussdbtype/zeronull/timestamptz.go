package zeronull

import (
	"database/sql/driver"
	"fmt"
	"time"

	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbtype"
)

type Timestamptz time.Time

func (Timestamptz) SkipUnderlyingTypePlan() {}

func (ts *Timestamptz) ScanTimestamptz(v gaussdbtype.Timestamptz) error {
	if !v.Valid {
		*ts = Timestamptz{}
		return nil
	}

	switch v.InfinityModifier {
	case gaussdbtype.Finite:
		*ts = Timestamptz(v.Time)
		return nil
	case gaussdbtype.Infinity:
		return fmt.Errorf("cannot scan Infinity into *time.Time")
	case gaussdbtype.NegativeInfinity:
		return fmt.Errorf("cannot scan -Infinity into *time.Time")
	default:
		return fmt.Errorf("invalid InfinityModifier: %v", v.InfinityModifier)
	}
}

func (ts Timestamptz) TimestamptzValue() (gaussdbtype.Timestamptz, error) {
	if time.Time(ts).IsZero() {
		return gaussdbtype.Timestamptz{}, nil
	}

	return gaussdbtype.Timestamptz{Time: time.Time(ts), Valid: true}, nil
}

// Scan implements the database/sql Scanner interface.
func (ts *Timestamptz) Scan(src any) error {
	if src == nil {
		*ts = Timestamptz{}
		return nil
	}

	var nullable gaussdbtype.Timestamptz
	err := nullable.Scan(src)
	if err != nil {
		return err
	}

	*ts = Timestamptz(nullable.Time)

	return nil
}

// Value implements the database/sql/driver Valuer interface.
func (ts Timestamptz) Value() (driver.Value, error) {
	if time.Time(ts).IsZero() {
		return nil, nil
	}

	return time.Time(ts), nil
}
