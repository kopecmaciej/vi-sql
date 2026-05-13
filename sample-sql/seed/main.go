// Seed generator for vi-sql sample databases.
// Writes one CSV per table to /tmp/vi-sql-seed/ (or $SEED_DIR).
// Fully deterministic — same output on every run.
//
// Usage: go run ./sample-sql/seed
package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var (
	outDir = func() string {
		if d := os.Getenv("SEED_DIR"); d != "" {
			return d
		}
		return "/tmp/vi-sql-seed"
	}()

	// Fixed anchor so reruns produce byte-identical CSVs.
	epoch = time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
)

func main() {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fatalf("mkdir: %v", err)
	}
	generateRoles()
	generatePermissions()
	generateRolePermissions()
	generateUsers()
	generateUserRoles()
	generateSessions()
	generateCategories()
	generateProducts()
	generateProductVariants()
	generateProductImages()
	generatePriceRules()
	generateCarriers()
	generateAddresses()
	generateCarts()
	generateCartItems()
	generateOrders()
	generateOrderItems()
	generatePayments()
	generateRefunds()
	generateShipments()
	generateShipmentEvents()
	generateAuditEvents()
	generateAPILogs()
	fmt.Printf("CSVs written to %s\n", outDir)
}

// ─── helpers ────────────────────────────────────────────────────────────────

func fatalf(f string, args ...any) {
	fmt.Fprintf(os.Stderr, f+"\n", args...)
	os.Exit(1)
}

// uuidFor produces a deterministic v4 UUID from a namespace + integer.
func uuidFor(ns string, i int) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", ns, i)))
	var b [16]byte
	copy(b[:], h[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 1
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		binary.BigEndian.Uint32(b[0:4]),
		binary.BigEndian.Uint16(b[4:6]),
		binary.BigEndian.Uint16(b[6:8]),
		binary.BigEndian.Uint16(b[8:10]),
		b[10:16])
}

func ts(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

func b01(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func itoa(n int) string { return strconv.Itoa(n) }

func writeCSV(name string, header []string, rows [][]string) {
	path := filepath.Join(outDir, name+".csv")
	f, err := os.Create(path)
	if err != nil {
		fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write(header); err != nil {
		fatalf("write header %s: %v", name, err)
	}
	if err := w.WriteAll(rows); err != nil {
		fatalf("write rows %s: %v", name, err)
	}
	w.Flush()
	if err := w.Error(); err != nil {
		fatalf("flush %s: %v", name, err)
	}
	fmt.Printf("  %-32s %d rows\n", name+".csv", len(rows))
}

// ─── shared lookup tables ────────────────────────────────────────────────────

var (
	firstNames = []string{"Alice", "Bob", "Carol", "Dan", "Eve", "Frank", "Grace", "Heidi", "Ivan", "Judy"}
	lastNames  = []string{"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis", "Wilson", "Moore"}
	sources    = []string{"organic", "referral", "paid_ad", "social"}
	locales    = []string{"en-US", "en-GB", "de-DE", "fr-FR", "es-ES", "pl-PL"}
	currencies = []string{"USD", "USD", "USD", "EUR", "GBP", "PLN"}
	coupons    = []string{"SUMMER10", "WELCOME20", "FLAT15", "FREESHIP", "VIP15"}
	cities     = []string{"New York", "Chicago", "Austin", "Seattle", "Miami"}
	cityStates = []string{"NY", "IL", "TX", "WA", "FL"}
	providers  = []string{"stripe", "stripe", "stripe", "paypal", "bank_transfer", "crypto"}

	brands     = []string{"BrandA", "BrandB", "BrandC", "OEM", "Premium"}
	colors     = []string{"black", "white", "red", "blue", "green", "silver"}
	materials  = []string{"plastic", "metal", "wood", "fabric", "leather"}
	origins    = []string{"CN", "DE", "US", "PL", "JP"}
	warranties = []int{6, 12, 24, 36}

	addrStreets   = []string{"Main St", "Oak Ave", "Maple Dr", "Broadway", "Park Lane"}
	addrCities    = []string{"New York", "Los Angeles", "Chicago", "Houston", "Phoenix", "Berlin", "Warsaw", "London", "Paris", "Amsterdam"}
	addrStates    = []string{"NY", "CA", "IL", "TX", "AZ", "", "", "", "", ""}
	addrCountries = []string{"US", "US", "US", "US", "DE", "PL", "GB", "FR", "US", "NL"}
	addrLabels    = []string{"Home", "Work", "Other"}

	agents = []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/121",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_2) Safari/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) Firefox/122",
		"okhttp/4.12.0",
		"PostmanRuntime/7.36.1",
	}

	apiAgents = []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/121",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 14_2) Safari/537.36",
		"curl/8.4.0",
		"PostmanRuntime/7.36.1",
		"python-requests/2.31.0",
		"axios/1.6.0",
	}
	apiMethods = []string{"GET", "GET", "GET", "GET", "POST", "PUT", "PATCH", "DELETE"}
	apiPaths   = []string{
		"/api/v1/products", "/api/v1/products/SKU-000001",
		"/api/v1/orders", "/api/v1/orders/latest",
		"/api/v1/users/me", "/api/v1/cart",
		"/api/v1/payments", "/api/v1/search",
		"/api/v1/categories", "/healthz",
	}
	apiCodes = []int{200, 200, 200, 201, 204, 304, 400, 401, 403, 404, 500}

	auditSchemas = []string{"auth", "catalog", "orders", "shipping"}
	auditTables  = []string{"users", "products", "orders", "payments", "shipments"}
	auditOps     = []string{"INSERT", "UPDATE", "UPDATE", "DELETE"}
)

const (
	numRegularUsers = 1000
	adminIdx        = 1001 // admin is user index 1001
)

func userUUID(gs int) string { return uuidFor("user", gs) }

func userStatus(gs int) string {
	switch {
	case gs <= 800:
		return "active"
	case gs <= 900:
		return "inactive"
	case gs <= 950:
		return "suspended"
	case gs <= 990:
		return "pending_verification"
	default:
		return "deleted"
	}
}

// activeUserUUIDs returns UUIDs of all active users ordered by email (gs 1..800 + admin).
func activeUserUUIDs() []string {
	uids := make([]string, 0, 801)
	for gs := 1; gs <= 800; gs++ {
		uids = append(uids, userUUID(gs))
	}
	uids = append(uids, userUUID(adminIdx))
	return uids
}

// ─── generators ─────────────────────────────────────────────────────────────

func generateRoles() {
	header := []string{"id", "name", "description", "created_at"}
	rows := [][]string{
		{"1", "admin", "Full system access", ts(epoch)},
		{"2", "staff", "Internal staff — read/write most resources", ts(epoch)},
		{"3", "support", "Customer support — read orders and users", ts(epoch)},
		{"4", "finance", "Finance team — read payments and issue refunds", ts(epoch)},
		{"5", "viewer", "Read-only access", ts(epoch)},
	}
	writeCSV("roles", header, rows)
}

func generatePermissions() {
	header := []string{"id", "resource", "action", "description"}
	rows := [][]string{
		{"1", "users", "read", "List and view users"},
		{"2", "users", "write", "Create and update users"},
		{"3", "users", "delete", "Delete or deactivate users"},
		{"4", "orders", "read", "View orders"},
		{"5", "orders", "write", "Create and update orders"},
		{"6", "orders", "cancel", "Cancel orders"},
		{"7", "products", "read", "View products"},
		{"8", "products", "write", "Create and update products"},
		{"9", "products", "delete", "Archive or delete products"},
		{"10", "payments", "read", "View payment records"},
		{"11", "payments", "refund", "Issue refunds"},
		{"12", "reports", "read", "Access reporting dashboards"},
		{"13", "settings", "write", "Modify system settings"},
	}
	writeCSV("permissions", header, rows)
}

func generateRolePermissions() {
	type perm struct {
		resource, action string
		id               int
	}
	perms := []perm{
		{"users", "read", 1}, {"users", "write", 2}, {"users", "delete", 3},
		{"orders", "read", 4}, {"orders", "write", 5}, {"orders", "cancel", 6},
		{"products", "read", 7}, {"products", "write", 8}, {"products", "delete", 9},
		{"payments", "read", 10}, {"payments", "refund", 11},
		{"reports", "read", 12}, {"settings", "write", 13},
	}
	header := []string{"role_id", "permission_id"}
	var rows [][]string

	// admin: all
	for _, p := range perms {
		rows = append(rows, []string{"1", itoa(p.id)})
	}
	// staff: orders/products/users × read/write
	staffR := map[string]bool{"orders": true, "products": true, "users": true}
	staffA := map[string]bool{"read": true, "write": true}
	for _, p := range perms {
		if staffR[p.resource] && staffA[p.action] {
			rows = append(rows, []string{"2", itoa(p.id)})
		}
	}
	// support: orders/users read + orders cancel
	for _, p := range perms {
		if (p.resource == "orders" || p.resource == "users") && p.action == "read" ||
			p.resource == "orders" && p.action == "cancel" {
			rows = append(rows, []string{"3", itoa(p.id)})
		}
	}
	// finance: payments/reports × read/refund
	finR := map[string]bool{"payments": true, "reports": true}
	finA := map[string]bool{"read": true, "refund": true}
	for _, p := range perms {
		if finR[p.resource] && finA[p.action] {
			rows = append(rows, []string{"4", itoa(p.id)})
		}
	}
	// viewer: all read
	for _, p := range perms {
		if p.action == "read" {
			rows = append(rows, []string{"5", itoa(p.id)})
		}
	}
	writeCSV("role_permissions", header, rows)
}

func generateUsers() {
	header := []string{
		"id", "email", "password_hash", "full_name", "phone",
		"status", "is_staff", "email_verified_at", "last_login_at", "last_login_ip",
		"failed_login_attempts", "locked_until", "metadata",
		"created_at", "updated_at", "deleted_at",
	}
	const pwHash = "$2a$08$zTkTHDFpzOxkJpVFB1A1YO0jnBbRVHWmRnKNSSamFYRobZKJkzS/."

	var rows [][]string
	for gs := 1; gs <= numRegularUsers; gs++ {
		phone := ""
		if gs%3 != 0 {
			phone = fmt.Sprintf("+1%d", 2000000000+gs*997)
		}
		emailVerified := ""
		if gs%10 != 0 {
			emailVerified = ts(epoch.AddDate(0, 0, -(gs % 500)))
		}
		lastLogin := ""
		if gs <= 900 {
			lastLogin = ts(epoch.AddDate(0, 0, -(gs % 30)))
		}
		metadata := fmt.Sprintf(`{"source":%q,"locale":%q,"newsletter":%v}`,
			sources[gs%4], locales[gs%6], gs%3 != 0)

		rows = append(rows, []string{
			userUUID(gs),
			fmt.Sprintf("user%d@example.com", gs),
			pwHash,
			firstNames[(gs-1)%10] + " " + lastNames[(gs-1)%10],
			phone,
			userStatus(gs),
			b01(gs <= 20),
			emailVerified,
			lastLogin,
			fmt.Sprintf("192.168.%d.%d", gs%255, (gs*7)%255),
			itoa(gs % 4),
			"", // locked_until
			metadata,
			ts(epoch), ts(epoch),
			"", // deleted_at
		})
	}
	// known admin
	rows = append(rows, []string{
		userUUID(adminIdx),
		"admin@example.com",
		"$2a$08$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi",
		"System Admin",
		"", "active", "1",
		ts(epoch), "", "",
		"0", "",
		`{"source":"direct","locale":"en-US","newsletter":false}`,
		ts(epoch), ts(epoch), "",
	})
	writeCSV("users", header, rows)
}

func generateUserRoles() {
	header := []string{"user_id", "role_id", "granted_at", "granted_by"}
	var rows [][]string

	// admin → admin role
	rows = append(rows, []string{userUUID(adminIdx), "1", ts(epoch), ""})
	// is_staff users (gs 1..20) → staff role
	for gs := 1; gs <= 20; gs++ {
		rows = append(rows, []string{userUUID(gs), "2", ts(epoch), ""})
	}
	// active non-staff users → viewer role (first 400)
	count := 0
	for gs := 21; gs <= 800 && count < 400; gs++ {
		rows = append(rows, []string{userUUID(gs), "5", ts(epoch), ""})
		count++
	}
	writeCSV("user_roles", header, rows)
}

func generateSessions() {
	header := []string{
		"id", "user_id", "token_hash", "ip_address", "user_agent",
		"last_seen_at", "expires_at", "revoked_at", "created_at",
	}
	activeUsers := activeUserUUIDs()
	var rows [][]string
	n := 0
outer:
	for _, uid := range activeUsers {
		for s := 0; s <= 1; s++ {
			if n >= 1500 {
				break outer
			}
			h := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", uid, s)))
			rows = append(rows, []string{
				uuidFor("session", n),
				uid,
				fmt.Sprintf("%x", h),
				fmt.Sprintf("10.0.%d.1", h[0]),
				agents[s%5],
				ts(epoch.Add(-time.Duration(s) * time.Hour)),
				ts(epoch.AddDate(0, 0, s*5+1)),
				"",
				ts(epoch),
			})
			n++
		}
	}
	writeCSV("sessions", header, rows)
}

func generateCategories() {
	header := []string{
		"id", "parent_id", "name", "slug", "description",
		"image_url", "position", "is_active", "created_at", "updated_at",
	}
	rows := [][]string{
		// top-level (parent_id empty = NULL)
		{"1", "", "Electronics", "electronics", "Electronic devices and accessories", "", "1", "1", ts(epoch), ts(epoch)},
		{"2", "", "Clothing", "clothing", "Apparel and fashion", "", "2", "1", ts(epoch), ts(epoch)},
		{"3", "", "Home & Garden", "home-garden", "Furniture, decor, and outdoor", "", "3", "1", ts(epoch), ts(epoch)},
		{"4", "", "Sports", "sports", "Sports and fitness equipment", "", "4", "1", ts(epoch), ts(epoch)},
		{"5", "", "Books", "books", "Books and digital media", "", "5", "1", ts(epoch), ts(epoch)},
		{"6", "", "Toys & Games", "toys-games", "Toys and games for all ages", "", "6", "1", ts(epoch), ts(epoch)},
		// sub-categories (IDs 7-19 = 13 leaf nodes)
		{"7", "1", "Smartphones", "smartphones", "", "", "1", "1", ts(epoch), ts(epoch)},
		{"8", "1", "Laptops", "laptops", "", "", "2", "1", ts(epoch), ts(epoch)},
		{"9", "1", "Audio", "audio", "", "", "3", "1", ts(epoch), ts(epoch)},
		{"10", "1", "Cameras", "cameras", "", "", "4", "1", ts(epoch), ts(epoch)},
		{"11", "2", "Men's", "mens", "", "", "1", "1", ts(epoch), ts(epoch)},
		{"12", "2", "Women's", "womens", "", "", "2", "1", ts(epoch), ts(epoch)},
		{"13", "2", "Kids", "kids-clothing", "", "", "3", "1", ts(epoch), ts(epoch)},
		{"14", "3", "Furniture", "furniture", "", "", "1", "1", ts(epoch), ts(epoch)},
		{"15", "3", "Kitchen", "kitchen", "", "", "2", "1", ts(epoch), ts(epoch)},
		{"16", "4", "Fitness", "fitness", "", "", "1", "1", ts(epoch), ts(epoch)},
		{"17", "4", "Outdoor", "outdoor", "", "", "2", "1", ts(epoch), ts(epoch)},
		{"18", "5", "Fiction", "fiction", "", "", "1", "1", ts(epoch), ts(epoch)},
		{"19", "5", "Non-Fiction", "non-fiction", "", "", "2", "1", ts(epoch), ts(epoch)},
	}
	writeCSV("categories", header, rows)
}

// productIDs[gs] for gs 1..500.
var productIDs [501]string

func generateProducts() {
	header := []string{
		"id", "category_id", "created_by", "sku", "name", "slug",
		"description", "short_desc", "status", "weight_kg",
		"attributes", "tags",
		"created_at", "updated_at",
		// search_vector and spec_sheet are Postgres-only; handled by UPDATE in sample.postgres.sql
	}
	const descPara = "Detailed product description paragraph with technical specifications. "
	tagsPool := []string{"sale", "new", "featured", "clearance", "bestseller"}
	tagCond := []func(int) bool{
		func(gs int) bool { return gs%4 == 0 },
		func(gs int) bool { return gs%6 == 0 },
		func(gs int) bool { return gs%10 == 0 },
		func(gs int) bool { return gs%15 == 0 },
		func(gs int) bool { return gs%8 == 0 },
	}

	var rows [][]string
	for gs := 1; gs <= 500; gs++ {
		productIDs[gs] = uuidFor("product", gs)

		// cycle through 13 leaf categories (IDs 7-19)
		catID := 7 + (gs-1)%13

		status := "active"
		if gs%20 == 0 {
			status = "draft"
		} else if gs%15 == 0 {
			status = "archived"
		}

		attrs := fmt.Sprintf(
			`{"brand":%q,"color":%q,"material":%q,"specs":{"warranty_months":%d,"country_of_origin":%q}}`,
			brands[gs%5], colors[gs%6], materials[gs%5], warranties[gs%4], origins[gs%5],
		)

		var tagList []string
		for i, cond := range tagCond {
			if cond(gs) {
				tagList = append(tagList, `"`+tagsPool[i]+`"`)
			}
		}
		tagsJSON := "{}"
		if len(tagList) > 0 {
			// Postgres TEXT[] literal — MySQL/SQLite see it as plain text
			tags := make([]string, len(tagList))
			for i, t := range tagList {
				tags[i] = strings.Trim(t, `"`)
			}
			tagsJSON = "{" + strings.Join(tags, ",") + "}"
		}

		rows = append(rows, []string{
			productIDs[gs],
			itoa(catID),
			"", // created_by: NULL
			fmt.Sprintf("SKU-%06d", gs),
			fmt.Sprintf("Product %d", gs),
			fmt.Sprintf("product-%d", gs),
			strings.Repeat(descPara, 15),
			fmt.Sprintf("Short description for product %d", gs),
			status,
			fmt.Sprintf("%.3f", 0.1+float64(gs%100)*0.1),
			attrs,
			tagsJSON,
			ts(epoch), ts(epoch),
		})
	}
	writeCSV("products", header, rows)
}

// variantIDs[i] for i 0..N-1, built in order of insertion.
var variantIDs []string

// variantPrices[i] mirrors the price for each variant (needed by cart_items).
var variantPrices []string

func generateProductVariants() {
	header := []string{
		"id", "product_id", "sku", "name", "attributes",
		"price", "compare_at_price", "cost_price",
		"stock_qty", "reserved_qty", "is_default",
		"created_at", "updated_at",
	}
	variantNames := []string{"Default", "Large", "XL"}
	variantIDs = nil
	variantPrices = nil
	var rows [][]string
	rn := 1
	for gs := 1; gs <= 500; gs++ {
		// SKU-000001…SKU-000500; last digit = gs%10.
		// Postgres condition: sku ~ '[0-4]$' → last digit ∈ {0,1,2,3,4}
		numV := 1
		if gs%10 <= 4 { // 0,1,2,3,4
			numV = 3
		}
		for v := 1; v <= numV; v++ {
			vid := uuidFor("variant", rn)
			variantIDs = append(variantIDs, vid)
			price := fmt.Sprintf("%.2f", 9.99+float64(rn%500)*1.97)
			variantPrices = append(variantPrices, price)

			compareAt := ""
			if v == 1 && rn%3 == 0 {
				compareAt = fmt.Sprintf("%.2f", 9.99+float64(rn%500)*1.97*1.2)
			}
			rows = append(rows, []string{
				vid,
				productIDs[gs],
				fmt.Sprintf("SKU-%06d-V%d", gs, v),
				variantNames[v-1],
				fmt.Sprintf(`{"size":%d,"color":%q}`, v, colors[gs%6]),
				price,
				compareAt,
				fmt.Sprintf("%.2f", 4.00+float64(rn%200)*0.9),
				itoa(rn % 150),
				"0",
				b01(v == 1),
				ts(epoch), ts(epoch),
			})
			rn++
		}
	}
	writeCSV("product_variants", header, rows)
}

func generateProductImages() {
	header := []string{
		"id", "product_id", "variant_id", "url", "alt_text",
		"position", "is_primary", "width", "height", "created_at",
	}
	var rows [][]string
	n := 0
	for gs := 1; gs <= 500; gs++ {
		// SKU length is always 10 (e.g. SKU-000001) → length%2 == 0 → 2 images
		for img := 1; img <= 2; img++ {
			rows = append(rows, []string{
				uuidFor("product_image", n),
				productIDs[gs],
				"", // variant_id: NULL
				fmt.Sprintf("https://cdn.example.com/products/SKU-%06d-%d.jpg", gs, img),
				fmt.Sprintf("Product %d - view %d", gs, img),
				itoa(img - 1),
				b01(img == 1),
				"", "", // width, height: NULL
				ts(epoch),
			})
			n++
		}
	}
	writeCSV("product_images", header, rows)
}

func generatePriceRules() {
	header := []string{
		"id", "name", "description", "code", "discount_type", "discount_value",
		"min_order_value", "max_uses", "uses_count", "starts_at", "ends_at",
		"is_active", "created_at",
	}
	rows := [][]string{
		{"1", "Summer Sale 10%", "", "SUMMER10", "percentage", "0.10", "50.00", "1000", "0",
			ts(epoch.AddDate(0, 0, -30)), ts(epoch.AddDate(0, 0, 60)), "1", ts(epoch)},
		{"2", "Welcome Discount", "", "WELCOME20", "percentage", "0.20", "", "", "0",
			ts(epoch.AddDate(0, 0, -90)), "", "1", ts(epoch)},
		{"3", "$15 Off", "", "FLAT15", "fixed", "15.00", "100.00", "500", "0",
			ts(epoch.AddDate(0, 0, -10)), ts(epoch.AddDate(0, 0, 20)), "1", ts(epoch)},
		{"4", "Free Shipping", "", "FREESHIP", "free_shipping", "0.00", "75.00", "", "0",
			ts(epoch), "", "1", ts(epoch)},
		{"5", "Clearance 30%", "", "CLEARANCE30", "percentage", "0.30", "200.00", "200", "0",
			ts(epoch.AddDate(0, 0, -5)), ts(epoch.AddDate(0, 0, 10)), "1", ts(epoch)},
		{"6", "VIP 15%", "", "VIP15", "percentage", "0.15", "", "", "0",
			ts(epoch.AddDate(0, 0, -180)), "", "1", ts(epoch)},
	}
	writeCSV("price_rules", header, rows)
}

func generateCarriers() {
	header := []string{"id", "name", "code", "tracking_url_template", "is_active"}
	rows := [][]string{
		{"1", "FedEx", "fedex", "https://www.fedex.com/tracking?tracknumbers={tracking}", "1"},
		{"2", "UPS", "ups", "https://www.ups.com/track?tracknum={tracking}", "1"},
		{"3", "DHL", "dhl", "https://www.dhl.com/track?tracking-id={tracking}", "1"},
		{"4", "USPS", "usps", "https://tools.usps.com/go/TrackConfirmAction?tLabels={tracking}", "1"},
		{"5", "GLS", "gls", "https://gls-group.eu/track/{tracking}", "1"},
	}
	writeCSV("carriers", header, rows)
}

// addressIDs and addressUserIDs indexed 0..N-1 (used by generateShipments later if needed).
var addressUserIDs []string

func generateAddresses() {
	header := []string{
		"id", "user_id", "label", "full_name", "line1", "line2",
		"city", "state", "postal_code", "country", "phone", "is_default",
		"created_at", "updated_at",
	}
	activeGS := make([]int, 0, 801)
	for gs := 1; gs <= 800; gs++ {
		activeGS = append(activeGS, gs)
	}
	activeGS = append(activeGS, adminIdx)

	addressUserIDs = nil
	var rows [][]string
	rn := 0
outerAddr:
	for _, gs := range activeGS {
		uid := userUUID(gs)
		fullName := firstNames[(gs-1)%10] + " " + lastNames[(gs-1)%10]
		if gs == adminIdx {
			fullName = "System Admin"
		}
		for g := 1; g <= 2; g++ {
			if rn >= 1600 {
				break outerAddr
			}
			rn++
			phone := ""
			if rn%2 == 0 {
				phone = fmt.Sprintf("+1%d", 2000000000+rn*997)
			}
			rows = append(rows, []string{
				uuidFor("address", rn),
				uid,
				addrLabels[(rn+g)%3],
				fullName,
				fmt.Sprintf("%d %s", (rn%999)+1, addrStreets[rn%5]),
				"",
				addrCities[rn%10],
				addrStates[rn%10],
				fmt.Sprintf("%05d", (rn%99999)+1),
				addrCountries[rn%10],
				phone,
				b01(g == 1),
				ts(epoch), ts(epoch),
			})
			addressUserIDs = append(addressUserIDs, uid)
		}
	}
	writeCSV("addresses", header, rows)
}

// cartIDs[gs] for gs 1..200.
var cartIDs [201]string

func generateCarts() {
	header := []string{
		"id", "user_id", "session_id", "metadata", "expires_at", "created_at", "updated_at",
	}
	activeUsers := activeUserUUIDs()
	var rows [][]string
	for gs := 1; gs <= 200; gs++ {
		cartIDs[gs] = uuidFor("cart", gs)
		userID, sessionID := "", ""
		if gs%3 == 0 {
			sessionID = fmt.Sprintf("sess_%08x", gs*997+12345)
		} else {
			userID = activeUsers[(gs-1)%len(activeUsers)]
		}
		rows = append(rows, []string{
			cartIDs[gs],
			userID,
			sessionID,
			fmt.Sprintf(`{"utm_source":%q}`, sources[gs%4]),
			ts(epoch.AddDate(0, 0, gs%14+1)),
			ts(epoch), ts(epoch),
		})
	}
	writeCSV("carts", header, rows)
}

func generateCartItems() {
	header := []string{"id", "cart_id", "variant_id", "quantity", "unit_price", "added_at"}
	numV := len(variantIDs)
	seen := make(map[string]bool)
	var rows [][]string
	n := 0
	for cRN := 1; cRN <= 200 && n < 400; cRN++ {
		cid := cartIDs[cRN]
		start := (cRN % 50)
		end := start + 2
		if end >= numV {
			end = numV - 1
		}
		for vIdx := start; vIdx <= end && n < 400; vIdx++ {
			vid := variantIDs[vIdx]
			key := cid + ":" + vid
			if seen[key] {
				continue
			}
			seen[key] = true
			rows = append(rows, []string{
				uuidFor("cart_item", n),
				cid, vid,
				itoa((cRN+vIdx)%4 + 1),
				variantPrices[vIdx],
				ts(epoch),
			})
			n++
		}
	}
	writeCSV("cart_items", header, rows)
}

// orderRows stores per-order data needed by later generators.
type orderData struct {
	id          string
	userID      string
	status      string
	confirmedAt time.Time
	cancelledAt time.Time
}

var orderRows [2001]orderData

func generateOrders() {
	header := []string{
		"id", "user_id", "status", "currency",
		"subtotal", "shipping_amount", "tax_amount", "discount_amount", "total_amount",
		// flat address columns (Postgres \copy will reconstruct composite type)
		"shipping_line1", "shipping_line2", "shipping_city", "shipping_state", "shipping_postal_code", "shipping_country",
		"billing_line1", "billing_line2", "billing_city", "billing_state", "billing_postal_code", "billing_country",
		"notes", "internal_notes", "metadata",
		"price_rule_id", "coupon_code", "ip_address",
		"confirmed_at", "cancelled_at", "cancellation_reason",
		"created_at", "updated_at",
	}
	activeUsers := activeUserUUIDs()
	numActive := len(activeUsers)
	cancelReasons := []string{"Customer request", "Out of stock", "Duplicate order", "Payment failed"}

	var rows [][]string
	for gs := 1; gs <= 2000; gs++ {
		oid := uuidFor("order", gs)
		uid := activeUsers[((gs*7)%numActive+numActive)%numActive]
		status := []string{"draft", "pending_payment", "paid", "processing", "shipped", "delivered", "cancelled", "refunded"}[gs%8]

		subtotal := fmt.Sprintf("%.2f", 10.00+float64(gs%490))
		shipping := "0.00"
		if gs%4 != 0 {
			shipping = fmt.Sprintf("%.2f", 5.00+float64(gs%20))
		}
		tax := fmt.Sprintf("%.2f", float64(gs%50)+1)
		discount := "0.00"
		if gs%5 == 0 {
			discount = fmt.Sprintf("%.2f", float64(gs%30))
		}
		sub, _ := strconv.ParseFloat(subtotal, 64)
		ship, _ := strconv.ParseFloat(shipping, 64)
		taxF, _ := strconv.ParseFloat(tax, 64)
		total := fmt.Sprintf("%.2f", sub+ship+taxF+1)

		notes := ""
		if gs%7 == 0 {
			notes = "Please handle with care. Fragile contents."
		}

		priceRuleID, couponCode := "", ""
		if gs%6 == 0 {
			priceRuleID = itoa(gs%5 + 1)
			couponCode = coupons[gs%5]
		}

		var confirmedAt, cancelledAt time.Time
		confirmedStr, cancelledStr, cancelReason := "", "", ""
		if gs%8 != 0 {
			confirmedAt = epoch.AddDate(0, 0, -(gs % 180))
			confirmedStr = ts(confirmedAt)
		}
		if gs%8 == 6 {
			cancelledAt = epoch.AddDate(0, 0, -(gs % 90))
			cancelledStr = ts(cancelledAt)
			cancelReason = cancelReasons[gs%4]
		}

		orderRows[gs] = orderData{oid, uid, status, confirmedAt, cancelledAt}

		rows = append(rows, []string{
			oid, uid, status, currencies[gs%6],
			subtotal, shipping, tax, discount, total,
			fmt.Sprintf("%d Main St", gs%999+1), "", cities[gs%5], cityStates[gs%5], fmt.Sprintf("%05d", gs%99999+1), "US",
			fmt.Sprintf("%d Billing Blvd", gs%999+1), "", cities[gs%5], cityStates[gs%5], fmt.Sprintf("%05d", gs%99999+1), "US",
			notes, "", "",
			priceRuleID, couponCode,
			fmt.Sprintf("203.%d.%d.%d", gs%255, (gs*3)%255, (gs*7)%255),
			confirmedStr, cancelledStr, cancelReason,
			ts(epoch), ts(epoch),
		})
	}
	writeCSV("orders", header, rows)
}

func generateOrderItems() {
	header := []string{
		"id", "order_id", "variant_id", "quantity",
		"unit_price", "total_price", "discount_amount",
		"tax_rate", "tax_amount", "product_snapshot", "created_at",
	}
	numV := len(variantIDs)
	var rows [][]string
	n := 0
	for gs := 1; gs <= 2000; gs++ {
		o := orderRows[gs]
		qty := gs%4 + 1
		start := (gs * 3) % numV
		end := start + 3
		if end > numV {
			end = numV
		}
		for vIdx := start; vIdx < end; vIdx++ {
			price := variantPrices[vIdx]
			priceF, _ := strconv.ParseFloat(price, 64)
			total := fmt.Sprintf("%.2f", priceF*float64(qty))
			taxAmt := fmt.Sprintf("%.2f", priceF*float64(qty)*0.2)
			snapshot := fmt.Sprintf(`{"sku":"SKU-VAR-%d","price":%s}`, vIdx+1, price)
			rows = append(rows, []string{
				uuidFor("order_item", n),
				o.id, variantIDs[vIdx],
				itoa(qty), price, total, "0.00",
				"0.2000", taxAmt, snapshot, ts(epoch),
			})
			n++
		}
	}
	writeCSV("order_items", header, rows)
}

// paymentData stores per-payment info needed by refunds.
type paymentData struct {
	id         string
	orderID    string
	status     string
	amount     string
	capturedAt time.Time
}

var allPayments []paymentData

func generatePayments() {
	header := []string{
		"id", "order_id", "provider", "status", "amount", "currency",
		"transaction_id", "provider_ref", "provider_data",
		"error_code", "error_message",
		"authorized_at", "captured_at", "failed_at",
		"refunded_amount", "created_at", "updated_at",
	}
	orderToPayStatus := map[string]string{
		"pending_payment": "pending",
		"paid":            "captured",
		"processing":      "captured",
		"shipped":         "captured",
		"delivered":       "captured",
		"cancelled":       "failed",
		"refunded":        "refunded",
		"draft":           "", // skip
	}
	countries := []string{"US", "DE", "GB", "PL", "FR"}
	allPayments = nil
	var rows [][]string
	rn := 0
	for gs := 1; gs <= 2000; gs++ {
		o := orderRows[gs]
		pStatus := orderToPayStatus[o.status]
		if pStatus == "" {
			continue
		}
		pid := uuidFor("payment", rn)
		amount := fmt.Sprintf("%.2f", 10.00+float64(gs%490)+float64(gs%20)+float64(gs%50)+1)

		h := sha256.Sum256([]byte(fmt.Sprintf("txn:%d", rn)))
		txnID := fmt.Sprintf("txn_%x", h[:12])
		provData := fmt.Sprintf(`{"gateway_version":"2023-10-16","risk_score":%d,"country":%q}`,
			rn%100, countries[rn%5])

		var authorizedAt, capturedAt, failedAt time.Time
		if o.status != "pending_payment" {
			authorizedAt = o.confirmedAt
		}
		if pStatus == "captured" || pStatus == "refunded" {
			capturedAt = o.confirmedAt.Add(2 * time.Minute)
		}
		if o.status == "cancelled" {
			failedAt = o.cancelledAt
		}

		allPayments = append(allPayments, paymentData{pid, o.id, pStatus, amount, capturedAt})
		rows = append(rows, []string{
			pid, o.id, providers[rn%6], pStatus, amount, currencies[gs%6],
			txnID, "", provData, "", "",
			ts(authorizedAt), ts(capturedAt), ts(failedAt),
			"0.00", ts(epoch), ts(epoch),
		})
		rn++
	}
	writeCSV("payments", header, rows)
}

func generateRefunds() {
	header := []string{
		"id", "payment_id", "amount", "reason", "status",
		"transaction_id", "processed_by", "created_at", "processed_at",
	}
	reasons := []string{"Customer request", "Item damaged", "Wrong item", "Out of stock", "Duplicate order"}
	staffUID := userUUID(1)
	var rows [][]string
	n := 0
	for i, p := range allPayments {
		if p.status != "refunded" {
			continue
		}
		rows = append(rows, []string{
			uuidFor("refund", n),
			p.id, p.amount,
			reasons[i%5], "completed", "",
			staffUID,
			ts(epoch),
			ts(p.capturedAt.Add(3 * 24 * time.Hour)),
		})
		n++
	}
	writeCSV("refunds", header, rows)
}

// shipmentData needed for shipment_events.
type shipmentData struct {
	id          string
	shippedAt   time.Time
	deliveredAt time.Time
}

var allShipments []shipmentData

func generateShipments() {
	header := []string{
		"id", "order_id", "carrier_id", "tracking_number", "status",
		"weight_kg", "dimensions", "label_url",
		"estimated_delivery", "shipped_at", "delivered_at",
		"created_at", "updated_at",
	}
	shipStatusMap := map[string]string{
		"shipped":    "in_transit",
		"delivered":  "delivered",
		"processing": "picked_up",
	}
	allShipments = nil
	var rows [][]string
	rn := 0
	for gs := 1; gs <= 2000; gs++ {
		o := orderRows[gs]
		sStatus, ok := shipStatusMap[o.status]
		if !ok {
			continue
		}
		sid := uuidFor("shipment", rn)
		h := sha256.Sum256([]byte(fmt.Sprintf("track:%d", rn)))
		tracking := strings.ToUpper(fmt.Sprintf("%x", h[:8]))

		var shippedAt, deliveredAt time.Time
		if o.status == "shipped" || o.status == "delivered" {
			shippedAt = o.confirmedAt.Add(48 * time.Hour)
		}
		if o.status == "delivered" {
			deliveredAt = o.confirmedAt.Add(6 * 24 * time.Hour)
		}

		allShipments = append(allShipments, shipmentData{sid, shippedAt, deliveredAt})
		rows = append(rows, []string{
			sid, o.id, itoa(rn%5 + 1), tracking, sStatus,
			fmt.Sprintf("%.3f", 0.1+float64(rn%100)*0.1),
			fmt.Sprintf(`{"width_cm":%.1f,"height_cm":%.1f,"depth_cm":%.1f}`,
				float64(rn%50+5), float64(rn%30+3), float64(rn%20+2)),
			"",
			ts(o.confirmedAt.AddDate(0, 0, rn%7+1)),
			ts(shippedAt), ts(deliveredAt),
			ts(epoch), ts(epoch),
		})
		rn++
	}
	writeCSV("shipments", header, rows)
}

func generateShipmentEvents() {
	header := []string{
		"id", "shipment_id", "status", "location", "message", "raw_data", "occurred_at", "created_at",
	}
	hubs := []string{"Chicago Hub", "New York Facility", "LA Depot", "Dallas Center", "Miami Hub"}
	sorts := []string{"Cincinnati Sort", "Memphis Hub", "Louisville Center", "Kansas City", "Denver"}

	var rows [][]string
	id := 0

	add := func(sid, status, loc, msg string, t time.Time) {
		id++
		rows = append(rows, []string{
			itoa(id), sid, status, loc, msg, "", ts(t), ts(epoch),
		})
	}

	for _, s := range allShipments {
		add(s.id, "pending", "Origin Warehouse", "Label created and shipment booked", epoch)
	}
	for i, s := range allShipments {
		if !s.shippedAt.IsZero() {
			add(s.id, "picked_up", hubs[i%5], "Package picked up by carrier", s.shippedAt)
		}
	}
	for i, s := range allShipments {
		if !s.shippedAt.IsZero() {
			add(s.id, "in_transit", sorts[i%5], "In transit to destination facility", s.shippedAt.Add(18*time.Hour))
		}
	}
	for _, s := range allShipments {
		if !s.deliveredAt.IsZero() {
			add(s.id, "delivered", "Destination", "Delivered — signed by recipient", s.deliveredAt)
		}
	}
	writeCSV("shipment_events", header, rows)
}

func generateAuditEvents() {
	header := []string{
		"id", "schema_name", "table_name", "operation", "row_id",
		"old_data", "new_data", "changed_fields",
		"performed_by", "ip_address", "occurred_at",
	}
	var rows [][]string
	for gs := 1; gs <= 5000; gs++ {
		oldData := ""
		if gs%3 != 0 {
			oldData = `{"status":"prev_value"}`
		}
		occurred := epoch.AddDate(0, 0, -(gs % 180)).Add(-time.Duration(gs%24) * time.Hour)
		rows = append(rows, []string{
			itoa(gs),
			auditSchemas[gs%4],
			auditTables[gs%5],
			auditOps[gs%4],
			uuidFor("audit_row", gs),
			oldData,
			`{"status":"new_value"}`,
			`{status,updated_at}`,
			userUUID(gs%20 + 1),
			fmt.Sprintf("10.0.%d.%d", gs%255, (gs*3)%255),
			ts(occurred),
		})
	}
	writeCSV("audit_events", header, rows)
}

func generateAPILogs() {
	header := []string{
		"id", "request_id", "method", "path", "query_params", "status_code",
		"duration_ms", "request_size", "response_size",
		"user_id", "ip_address", "user_agent", "error_message", "created_at",
	}
	activeUsers := activeUserUUIDs()
	var rows [][]string
	for gs := 1; gs <= 10000; gs++ {
		queryParams := ""
		if gs%4 == 0 {
			queryParams = fmt.Sprintf(`{"page":%d,"per_page":25,"sort":"created_at"}`, gs%20)
		}
		userID := ""
		if gs%8 != 0 {
			userID = activeUsers[gs%len(activeUsers)]
		}
		errMsg := ""
		if gs%11 == 10 {
			errMsg = fmt.Sprintf("Internal server error — see logs for trace %d", gs)
		}
		occurred := epoch.AddDate(0, 0, -(gs % 90)).
			Add(-time.Duration(gs%24) * time.Hour).
			Add(-time.Duration(gs%60) * time.Minute)
		rows = append(rows, []string{
			itoa(gs),
			uuidFor("api_log_req", gs),
			apiMethods[gs%8],
			apiPaths[gs%10],
			queryParams,
			itoa(apiCodes[gs%11]),
			itoa(gs%995 + 5),
			itoa(gs % 4096),
			itoa(gs % 65536),
			userID,
			fmt.Sprintf("34.%d.%d.%d", gs%255, (gs*3)%255, (gs*7)%255),
			apiAgents[gs%6],
			errMsg,
			ts(occurred),
		})
	}
	writeCSV("api_logs", header, rows)
}
