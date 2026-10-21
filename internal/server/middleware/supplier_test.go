package middleware

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// DS-017, DS-053: роль supplier — «по умолчанию запрещено»; открыты только кабинет, выход,
// свой профиль и смена своего пояса.
func TestSupplierAllowList(t *testing.T) {
	allowed := [][2]string{
		{"GET", "/api/v1/supplier/purchase-orders"},
		{"PUT", "/api/v1/supplier/purchase-orders/x/items/y"},
		{"POST", "/api/v1/auth/logout"},
		{"GET", "/api/v1/auth/me"},
		{"PUT", "/api/v1/users/me/timezone"},
	}
	for _, a := range allowed {
		assert.True(t, supplierAPIAllowed(a[0], a[1]), a[1])
	}
	denied := [][2]string{
		{"GET", "/api/v1/purchase-orders"},
		{"GET", "/api/v1/users"},
		{"GET", "/api/v1/users/me/timezone"},
		{"GET", "/api/v1/stock"},
		{"GET", "/api/v1/suppliers"},
		{"GET", "/api/v1/supplierx"},
	}
	for _, d := range denied {
		assert.False(t, supplierAPIAllowed(d[0], d[1]), d[1])
	}
	assert.True(t, SupplierSSRAllowed("/supplier/orders"))
	assert.True(t, SupplierSSRAllowed("/supplier"))
	assert.False(t, SupplierSSRAllowed("/suppliers"))
	assert.False(t, SupplierSSRAllowed("/"))
	assert.False(t, SupplierSSRAllowed("/purchases"))
}
