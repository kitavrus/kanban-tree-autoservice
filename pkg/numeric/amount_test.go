package numeric

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAmount_MarshalJSON_EmptyToZero(t *testing.T) {
	b, err := Amount("").MarshalJSON()
	require.NoError(t, err)
	assert.Equal(t, "0", string(b))

	b, err = Amount("12.50").MarshalJSON()
	require.NoError(t, err)
	assert.Equal(t, "12.50", string(b))
}

func TestAmount_UnmarshalJSON_RejectsNonNumber(t *testing.T) {
	var a Amount
	for _, raw := range []string{`"12.50"`, `{}`, `[]`, ``} {
		err := a.UnmarshalJSON([]byte(raw))
		assert.Error(t, err, "raw=%q", raw)
	}
}

func TestAmount_UnmarshalJSON_Null(t *testing.T) {
	var a Amount = "should be reset"
	require.NoError(t, a.UnmarshalJSON([]byte("null")))
	assert.Equal(t, Amount(""), a)

	var b Amount
	require.NoError(t, json.Unmarshal([]byte(`5.5`), &b))
	assert.Equal(t, Amount("5.5"), b)
}

func TestAmount_Validate_TooLarge(t *testing.T) {
	scale := Scale{IntDigits: 9, FracDigits: 3}
	assert.Equal(t, MsgTooLarge, Amount("1000000000").Validate(scale, true))
	assert.Empty(t, Amount("999999999").Validate(scale, true), "граница — валидно")
}

func TestAmount_Validate_TooPrecise(t *testing.T) {
	scale := Scale{IntDigits: 10, FracDigits: 2}
	assert.Equal(t, MsgTooPrecise, Amount("1.239").Validate(scale, false))
	assert.Empty(t, Amount("1.23").Validate(scale, false), "граница — валидно")
}

func TestAmount_Validate_InvalidFormat(t *testing.T) {
	scale := Scale{IntDigits: 9, FracDigits: 3}
	for _, v := range []string{"abc", "1e5", "NaN", "1.2.3"} {
		assert.Equal(t, MsgInvalidFormat, Amount(v).Validate(scale, true), "v=%q", v)
	}
}

func TestAmount_Validate_NegativeNotAllowed(t *testing.T) {
	scale := Scale{IntDigits: 9, FracDigits: 3}
	assert.Equal(t, MsgInvalid, Amount("-1").Validate(scale, false))
	assert.Empty(t, Amount("-1").Validate(scale, true), "allowNegative=true — валидно")
}
