package platform

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var whatsappPattern = regexp.MustCompile(`^\+?[1-9][0-9]{6,14}$`)

type businessView struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	Description  string `json:"description"`
	Whatsapp     string `json:"whatsapp"`
	Email        string `json:"email"`
	Currency     string `json:"currency"`
	Timezone     string `json:"timezone"`
	ThemeColor   string `json:"theme_color"`
	Published    bool   `json:"published"`
	Plan         string `json:"plan"`
	InstagramURL string `json:"instagram_url"`
	FacebookURL  string `json:"facebook_url"`
	TikTokURL    string `json:"tiktok_url"`
	WebsiteURL   string `json:"website_url"`
}
type businessUpdate struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	Whatsapp     string `json:"whatsapp"`
	Email        string `json:"email"`
	Currency     string `json:"currency"`
	Timezone     string `json:"timezone"`
	ThemeColor   string `json:"theme_color"`
	Published    bool   `json:"published"`
	InstagramURL string `json:"instagram_url"`
	FacebookURL  string `json:"facebook_url"`
	TikTokURL    string `json:"tiktok_url"`
	WebsiteURL   string `json:"website_url"`
}

func validSocialURL(raw, domain string) bool {
	if raw == "" {
		return true
	}
	if len(raw) > 500 {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Hostname() == "" {
		return false
	}
	if domain == "" {
		return true
	}
	host := strings.ToLower(u.Hostname())
	return host == domain || host == "www."+domain
}

func (a *App) readBusiness(r *http.Request, id string) (businessView, error) {
	var b businessView
	err := a.DB.QueryRow(r.Context(), `SELECT id::text,name,slug,description,whatsapp,email,currency,timezone,theme_color,published,CASE WHEN plan_expires_at>now() THEN plan ELSE 'free' END,instagram_url,facebook_url,tiktok_url,website_url FROM businesses WHERE id=$1`, id).Scan(&b.ID, &b.Name, &b.Slug, &b.Description, &b.Whatsapp, &b.Email, &b.Currency, &b.Timezone, &b.ThemeColor, &b.Published, &b.Plan, &b.InstagramURL, &b.FacebookURL, &b.TikTokURL, &b.WebsiteURL)
	return b, err
}
func (a *App) business(w http.ResponseWriter, r *http.Request) {
	b, err := a.readBusiness(r, who(r).BusinessID)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	writeJSON(w, 200, b)
}
func (a *App) updateBusiness(w http.ResponseWriter, r *http.Request) {
	var in businessUpdate
	if !decode(w, r, &in) {
		return
	}
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	in.Whatsapp = strings.TrimSpace(in.Whatsapp)
	in.Email = strings.TrimSpace(in.Email)
	in.InstagramURL = strings.TrimSpace(in.InstagramURL)
	in.FacebookURL = strings.TrimSpace(in.FacebookURL)
	in.TikTokURL = strings.TrimSpace(in.TikTokURL)
	in.WebsiteURL = strings.TrimSpace(in.WebsiteURL)
	if len(in.Name) < 2 || len(in.Name) > 120 || len(in.Description) > 2000 || len(in.Whatsapp) > 30 || len(in.Email) > 254 || len(in.Timezone) > 80 || len(in.ThemeColor) != 7 || in.ThemeColor[0] != '#' {
		fail(w, 400, "Invalid business details")
		return
	}
	if in.Email != "" && !validEmail(in.Email) {
		fail(w, 400, "Invalid email")
		return
	}
	if len(in.Currency) != 3 {
		fail(w, 400, "Currency must be a three-letter code")
		return
	}
	for _, c := range in.Currency {
		if c < 'A' || c > 'Z' {
			fail(w, 400, "Currency must be a three-letter code")
			return
		}
	}
	if in.Whatsapp != "" && !whatsappPattern.MatchString(in.Whatsapp) {
		fail(w, 400, "WhatsApp number must use international digits")
		return
	}
	if !validSocialURL(in.InstagramURL, "instagram.com") || !validSocialURL(in.FacebookURL, "facebook.com") || !validSocialURL(in.TikTokURL, "tiktok.com") || !validSocialURL(in.WebsiteURL, "") {
		fail(w, 400, "Social links must be valid HTTPS URLs")
		return
	}
	for _, c := range in.ThemeColor[1:] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			fail(w, 400, "Invalid theme color")
			return
		}
	}
	if _, err := time.LoadLocation(in.Timezone); err != nil {
		fail(w, 400, "Invalid timezone")
		return
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		unavailable(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	var oldCurrency string
	err = tx.QueryRow(r.Context(), `SELECT currency FROM businesses WHERE id=$1 FOR UPDATE`, who(r).BusinessID).Scan(&oldCurrency)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	if oldCurrency != in.Currency {
		var used bool
		err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM products WHERE business_id=$1) OR EXISTS(SELECT 1 FROM invoices WHERE business_id=$1)`, who(r).BusinessID).Scan(&used)
		if err != nil {
			unavailable(w, r, err)
			return
		}
		if used {
			fail(w, 409, "Currency cannot change after products or invoices exist")
			return
		}
	}
	_, err = tx.Exec(r.Context(), `UPDATE businesses SET name=$1,description=$2,whatsapp=$3,email=$4,currency=$5,timezone=$6,theme_color=$7,published=$8,instagram_url=$9,facebook_url=$10,tiktok_url=$11,website_url=$12 WHERE id=$13`, in.Name, in.Description, in.Whatsapp, in.Email, in.Currency, in.Timezone, in.ThemeColor, in.Published, in.InstagramURL, in.FacebookURL, in.TikTokURL, in.WebsiteURL, who(r).BusinessID)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		unavailable(w, r, err)
		return
	}
	a.business(w, r)
}

type product struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Kind        string `json:"kind"`
	PriceMinor  int64  `json:"price_minor"`
	Active      bool   `json:"active"`
	HasImage    bool   `json:"has_image"`
}

func (a *App) listProducts(r *http.Request, businessID string, onlyActive bool) ([]product, error) {
	rows, err := a.DB.Query(r.Context(), `SELECT id::text,name,description,kind,price_minor,active,image_id IS NOT NULL FROM products WHERE business_id=$1 AND ($2=false OR active=true) ORDER BY created_at DESC LIMIT 200`, businessID, onlyActive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []product{}
	for rows.Next() {
		var p product
		if err = rows.Scan(&p.ID, &p.Name, &p.Description, &p.Kind, &p.PriceMinor, &p.Active, &p.HasImage); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (a *App) products(w http.ResponseWriter, r *http.Request) {
	page, ok := requestPage(w, r)
	if !ok {
		return
	}
	rows, err := a.DB.Query(r.Context(), `SELECT id::text,name,description,kind,price_minor,active,image_id IS NOT NULL FROM products WHERE business_id=$1 ORDER BY created_at DESC,id DESC LIMIT $2 OFFSET $3`, who(r).BusinessID, listPageSize+1, (page-1)*listPageSize)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	defer rows.Close()
	out := []product{}
	for rows.Next() {
		var p product
		if err = rows.Scan(&p.ID, &p.Name, &p.Description, &p.Kind, &p.PriceMinor, &p.Active, &p.HasImage); err != nil {
			unavailable(w, r, err)
			return
		}
		out = append(out, p)
	}
	if err = rows.Err(); err != nil {
		unavailable(w, r, err)
		return
	}
	writeJSON(w, 200, paged(out, page))
}
func planProductLimit(plan string) int {
	switch plan {
	case "starter":
		return 30
	case "business":
		return 100
	default:
		return 5
	}
}
func validProduct(p product) bool {
	return len(strings.TrimSpace(p.Name)) >= 2 && len(p.Name) <= 160 && len(p.Description) <= 2000 && (p.Kind == "product" || p.Kind == "service") && p.PriceMinor >= 0 && p.PriceMinor <= 100000000000
}
func (a *App) createProduct(w http.ResponseWriter, r *http.Request) {
	var in product
	if !decode(w, r, &in) {
		return
	}
	if !validProduct(in) {
		fail(w, 400, "Invalid product details")
		return
	}
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		unavailable(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	var plan string
	var count int
	if err = tx.QueryRow(r.Context(), `SELECT CASE WHEN plan_expires_at>now() THEN plan ELSE 'free' END FROM businesses WHERE id=$1 FOR UPDATE`, who(r).BusinessID).Scan(&plan); err != nil {
		unavailable(w, r, err)
		return
	}
	if err = tx.QueryRow(r.Context(), `SELECT count(*) FROM products WHERE business_id=$1`, who(r).BusinessID).Scan(&count); err != nil {
		unavailable(w, r, err)
		return
	}
	if count >= planProductLimit(plan) {
		fail(w, 403, "Product limit reached for your plan")
		return
	}
	in.ID = newID()
	_, err = tx.Exec(r.Context(), `INSERT INTO products(id,business_id,name,description,kind,price_minor,active) VALUES($1,$2,$3,$4,$5,$6,$7)`, in.ID, who(r).BusinessID, strings.TrimSpace(in.Name), in.Description, in.Kind, in.PriceMinor, in.Active)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		unavailable(w, r, err)
		return
	}
	writeJSON(w, 201, in)
}
func (a *App) updateProduct(w http.ResponseWriter, r *http.Request) {
	var in product
	if !decode(w, r, &in) {
		return
	}
	if !validProduct(in) {
		fail(w, 400, "Invalid product details")
		return
	}
	id := chi.URLParam(r, "id")
	if !validID(id) {
		fail(w, 404, "Product not found")
		return
	}
	ct, err := a.DB.Exec(r.Context(), `UPDATE products SET name=$1,description=$2,kind=$3,price_minor=$4,active=$5 WHERE id=$6 AND business_id=$7`, strings.TrimSpace(in.Name), in.Description, in.Kind, in.PriceMinor, in.Active, id, who(r).BusinessID)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	if ct.RowsAffected() == 0 {
		fail(w, 404, "Product not found")
		return
	}
	in.ID = id
	writeJSON(w, 200, in)
}
func (a *App) deleteProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validID(id) {
		fail(w, 404, "Product not found")
		return
	}
	var imageID, imageExt *string
	err := a.DB.QueryRow(r.Context(), `DELETE FROM products WHERE id=$1 AND business_id=$2 RETURNING image_id::text,image_ext`, id, who(r).BusinessID).Scan(&imageID, &imageExt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			fail(w, 404, "Product not found")
			return
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			fail(w, 409, "Product has enquiries; archive it instead")
		} else {
			unavailable(w, r, err)
		}
		return
	}
	if imageID != nil && imageExt != nil {
		_ = os.Remove(imagePath(*imageID, *imageExt))
	}
	w.WriteHeader(204)
}
func (a *App) publicStore(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	var b businessView
	err := a.DB.QueryRow(r.Context(), `SELECT id::text,name,slug,description,whatsapp,email,currency,timezone,theme_color,published,CASE WHEN plan_expires_at>now() THEN plan ELSE 'free' END,instagram_url,facebook_url,tiktok_url,website_url FROM businesses WHERE slug=$1 AND published=true`, slug).Scan(&b.ID, &b.Name, &b.Slug, &b.Description, &b.Whatsapp, &b.Email, &b.Currency, &b.Timezone, &b.ThemeColor, &b.Published, &b.Plan, &b.InstagramURL, &b.FacebookURL, &b.TikTokURL, &b.WebsiteURL)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			fail(w, 404, "Storefront not found")
		} else {
			unavailable(w, r, err)
		}
		return
	}
	p, err := a.listProducts(r, b.ID, true)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	writeJSON(w, 200, map[string]any{"business": b, "products": p})
}

type customer struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
	Notes string `json:"notes"`
}

func (a *App) customers(w http.ResponseWriter, r *http.Request) {
	page, ok := requestPage(w, r)
	if !ok {
		return
	}
	rows, err := a.DB.Query(r.Context(), `SELECT id::text,name,email,phone,notes FROM customers WHERE business_id=$1 ORDER BY created_at DESC,id DESC LIMIT $2 OFFSET $3`, who(r).BusinessID, listPageSize+1, (page-1)*listPageSize)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	defer rows.Close()
	out := []customer{}
	for rows.Next() {
		var c customer
		if err = rows.Scan(&c.ID, &c.Name, &c.Email, &c.Phone, &c.Notes); err != nil {
			unavailable(w, r, err)
			return
		}
		out = append(out, c)
	}
	if rows.Err() != nil {
		unavailable(w, r, rows.Err())
		return
	}
	writeJSON(w, 200, paged(out, page))
}
func (a *App) createCustomer(w http.ResponseWriter, r *http.Request) {
	var c customer
	if !decode(w, r, &c) {
		return
	}
	c.Name = strings.TrimSpace(c.Name)
	c.Email = strings.TrimSpace(c.Email)
	if len(c.Name) < 2 || len(c.Name) > 160 || len(c.Phone) > 40 || len(c.Notes) > 2000 || (c.Email != "" && !validEmail(c.Email)) {
		fail(w, 400, "Invalid customer details")
		return
	}
	c.ID = newID()
	_, err := a.DB.Exec(r.Context(), `INSERT INTO customers(id,business_id,name,email,phone,notes) VALUES($1,$2,$3,$4,$5,$6)`, c.ID, who(r).BusinessID, c.Name, c.Email, c.Phone, c.Notes)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	writeJSON(w, 201, c)
}

type enquiry struct {
	ID        string `json:"id"`
	ProductID string `json:"product_id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}

func (a *App) publicEnquiry(w http.ResponseWriter, r *http.Request) {
	var e enquiry
	if !decode(w, r, &e) {
		return
	}
	e.Name = strings.TrimSpace(e.Name)
	e.Email = strings.TrimSpace(e.Email)
	e.Message = strings.TrimSpace(e.Message)
	if len(e.Name) < 2 || len(e.Name) > 160 || len(e.Phone) > 40 || len(e.Message) < 5 || len(e.Message) > 3000 || (e.Email == "" && e.Phone == "") || (e.Email != "" && !validEmail(e.Email)) {
		fail(w, 400, "Invalid enquiry")
		return
	}
	var businessID string
	if err := a.DB.QueryRow(r.Context(), `SELECT id::text FROM businesses WHERE slug=$1 AND published=true`, chi.URLParam(r, "slug")).Scan(&businessID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			fail(w, 404, "Storefront not found")
		} else {
			unavailable(w, r, err)
		}
		return
	}
	var productID any = nil
	if e.ProductID != "" {
		if !validID(e.ProductID) {
			fail(w, 400, "Invalid product selected")
			return
		}
		productID = e.ProductID
	}
	e.ID = newID()
	_, err := a.DB.Exec(r.Context(), `INSERT INTO enquiries(id,business_id,product_id,name,email,phone,message) VALUES($1,$2,$3,$4,$5,$6,$7)`, e.ID, businessID, productID, e.Name, e.Email, e.Phone, e.Message)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			fail(w, 400, "Invalid product selected")
		} else {
			unavailable(w, r, err)
		}
		return
	}
	writeJSON(w, 201, map[string]string{"id": e.ID})
}
func (a *App) enquiries(w http.ResponseWriter, r *http.Request) {
	page, ok := requestPage(w, r)
	if !ok {
		return
	}
	rows, err := a.DB.Query(r.Context(), `SELECT id::text,COALESCE(product_id::text,''),name,email,phone,message,created_at::text FROM enquiries WHERE business_id=$1 ORDER BY created_at DESC,id DESC LIMIT $2 OFFSET $3`, who(r).BusinessID, listPageSize+1, (page-1)*listPageSize)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	defer rows.Close()
	out := []enquiry{}
	for rows.Next() {
		var e enquiry
		if err = rows.Scan(&e.ID, &e.ProductID, &e.Name, &e.Email, &e.Phone, &e.Message, &e.CreatedAt); err != nil {
			unavailable(w, r, err)
			return
		}
		out = append(out, e)
	}
	if rows.Err() != nil {
		unavailable(w, r, rows.Err())
		return
	}
	writeJSON(w, 200, paged(out, page))
}
