package gaussdbtype_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbtype"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbxtest"
	"github.com/stretchr/testify/assert"
)

func TestTimeCodec(t *testing.T) {
	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, nil, "time", []gaussdbxtest.ValueRoundTripTest{
		{
			gaussdbtype.Time{Microseconds: 0, Valid: true},
			new(gaussdbtype.Time),
			isExpectedEq(gaussdbtype.Time{Microseconds: 0, Valid: true}),
		},
		{
			gaussdbtype.Time{Microseconds: 1, Valid: true},
			new(gaussdbtype.Time),
			isExpectedEq(gaussdbtype.Time{Microseconds: 1, Valid: true}),
		},
		{
			gaussdbtype.Time{Microseconds: 86399999999, Valid: true},
			new(gaussdbtype.Time),
			isExpectedEq(gaussdbtype.Time{Microseconds: 86399999999, Valid: true}),
		},
		{
			gaussdbtype.Time{Microseconds: 86400000000, Valid: true},
			new(gaussdbtype.Time),
			isExpectedEq(gaussdbtype.Time{Microseconds: 86400000000, Valid: true}),
		},
		{
			time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
			new(gaussdbtype.Time),
			isExpectedEq(gaussdbtype.Time{Microseconds: 0, Valid: true}),
		},
		{
			gaussdbtype.Time{Microseconds: 0, Valid: true},
			new(time.Time),
			isExpectedEq(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)),
		},
		{gaussdbtype.Time{}, new(gaussdbtype.Time), isExpectedEq(gaussdbtype.Time{})},
		{nil, new(gaussdbtype.Time), isExpectedEq(gaussdbtype.Time{})},
	})
}

func TestTimeTextScanner(t *testing.T) {
	var gaussdbTime gaussdbtype.Time

	assert.NoError(t, gaussdbTime.Scan("07:37:16"))
	assert.Equal(t, true, gaussdbTime.Valid)
	assert.Equal(t, int64(7*time.Hour+37*time.Minute+16*time.Second), gaussdbTime.Microseconds*int64(time.Microsecond))

	assert.NoError(t, gaussdbTime.Scan("15:04:05"))
	assert.Equal(t, true, gaussdbTime.Valid)
	assert.Equal(t, int64(15*time.Hour+4*time.Minute+5*time.Second), gaussdbTime.Microseconds*int64(time.Microsecond))

	// parsing of fractional digits
	assert.NoError(t, gaussdbTime.Scan("15:04:05.00"))
	assert.Equal(t, true, gaussdbTime.Valid)
	assert.Equal(t, int64(15*time.Hour+4*time.Minute+5*time.Second), gaussdbTime.Microseconds*int64(time.Microsecond))

	const mirco = "789123"
	const woFraction = int64(4*time.Hour + 5*time.Minute + 6*time.Second) // time without fraction
	for i := 0; i <= len(mirco); i++ {
		assert.NoError(t, gaussdbTime.Scan("04:05:06."+mirco[:i]))
		assert.Equal(t, true, gaussdbTime.Valid)

		frac, _ := strconv.ParseInt(mirco[:i], 10, 64)
		for k := i; k < 6; k++ {
			frac *= 10
		}
		assert.Equal(t, woFraction+frac*int64(time.Microsecond), gaussdbTime.Microseconds*int64(time.Microsecond))
	}

	// parsing of too long fraction errors
	assert.Error(t, gaussdbTime.Scan("04:05:06.7891234"))
	assert.Equal(t, false, gaussdbTime.Valid)
	assert.Equal(t, int64(0), gaussdbTime.Microseconds)

	// parsing of timetz errors
	assert.Error(t, gaussdbTime.Scan("04:05:06.789-08"))
	assert.Equal(t, false, gaussdbTime.Valid)
	assert.Equal(t, int64(0), gaussdbTime.Microseconds)

	assert.Error(t, gaussdbTime.Scan("04:05:06-08:00"))
	assert.Equal(t, false, gaussdbTime.Valid)
	assert.Equal(t, int64(0), gaussdbTime.Microseconds)

	// parsing of date errors
	assert.Error(t, gaussdbTime.Scan("1997-12-17"))
	assert.Equal(t, false, gaussdbTime.Valid)
	assert.Equal(t, int64(0), gaussdbTime.Microseconds)

	// parsing of text errors
	assert.Error(t, gaussdbTime.Scan("12345678"))
	assert.Equal(t, false, gaussdbTime.Valid)
	assert.Equal(t, int64(0), gaussdbTime.Microseconds)

	assert.Error(t, gaussdbTime.Scan("12-34-56"))
	assert.Equal(t, false, gaussdbTime.Valid)
	assert.Equal(t, int64(0), gaussdbTime.Microseconds)

	assert.Error(t, gaussdbTime.Scan("12:34-56"))
	assert.Equal(t, false, gaussdbTime.Valid)
	assert.Equal(t, int64(0), gaussdbTime.Microseconds)

	assert.Error(t, gaussdbTime.Scan("12-34:56"))
	assert.Equal(t, false, gaussdbTime.Valid)
	assert.Equal(t, int64(0), gaussdbTime.Microseconds)
}
