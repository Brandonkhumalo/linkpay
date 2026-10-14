package platform

import (
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

const maxImageBytes = 3 << 20

func planImageLimit(plan string) int64 {
	switch plan {
	case "starter":
		return 150 << 20
	case "business":
		return 500 << 20
	default:
		return 25 << 20
	}
}

func imagePath(id, ext string) string {
	dir := os.Getenv("MEDIA_DIR")
	if dir == "" {
		dir = "../../media"
	}
	return filepath.Join(dir, id+"."+ext)
}

func (a *App) uploadProductImage(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validID(id) {
		fail(w, 404, "Product not found")
		return
	}
	var exists bool
	if err := a.DB.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM products WHERE id=$1 AND business_id=$2)`, id, who(r).BusinessID).Scan(&exists); err != nil {
		unavailable(w, r, err)
		return
	}
	if !exists {
		fail(w, 404, "Product not found")
		return
	}
	tempName, ext, size, ok := receiveImage(w, r)
	if !ok {
		return
	}
	defer os.Remove(tempName)
	newImageID := newID()
	target := imagePath(newImageID, ext)
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		unavailable(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	var plan string
	var logoSize int64
	if err = tx.QueryRow(r.Context(), `SELECT CASE WHEN plan_expires_at>now() THEN plan ELSE 'free' END,logo_size FROM businesses WHERE id=$1 FOR UPDATE`, who(r).BusinessID).Scan(&plan, &logoSize); err != nil {
		unavailable(w, r, err)
		return
	}
	var productOldID, productOldExt *string
	var productOldSize int64
	err = tx.QueryRow(r.Context(), `SELECT image_id::text,image_ext,image_size FROM products WHERE id=$1 AND business_id=$2 FOR UPDATE`, id, who(r).BusinessID).Scan(&productOldID, &productOldExt, &productOldSize)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "Product not found")
		return
	}
	if err != nil {
		unavailable(w, r, err)
		return
	}
	var used int64
	if err = tx.QueryRow(r.Context(), `SELECT COALESCE(sum(image_size),0) FROM products WHERE business_id=$1`, who(r).BusinessID).Scan(&used); err != nil {
		unavailable(w, r, err)
		return
	}
	if used+logoSize-productOldSize+size > planImageLimit(plan) {
		fail(w, 403, "Image storage allowance reached")
		return
	}
	if err = os.Rename(tempName, target); err != nil {
		unavailable(w, r, err)
		return
	}
	if _, err = tx.Exec(r.Context(), `UPDATE products SET image_id=$1,image_ext=$2,image_size=$3 WHERE id=$4 AND business_id=$5`, newImageID, ext, size, id, who(r).BusinessID); err != nil {
		os.Remove(target)
		unavailable(w, r, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		os.Remove(target)
		unavailable(w, r, err)
		return
	}
	if productOldID != nil && productOldExt != nil {
		_ = os.Remove(imagePath(*productOldID, *productOldExt))
	}
	writeJSON(w, 200, map[string]any{"has_image": true})
}

func receiveImage(w http.ResponseWriter, r *http.Request) (string, string, int64, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImageBytes+65536)
	if err := r.ParseMultipartForm(maxImageBytes); err != nil {
		fail(w, 400, "Image must be under 3 MB")
		return "", "", 0, false
	}
	file, header, err := r.FormFile("image")
	if err != nil || header.Size > maxImageBytes {
		fail(w, 400, "Choose a JPEG or PNG under 3 MB")
		return "", "", 0, false
	}
	defer file.Close()
	config, format, err := image.DecodeConfig(file)
	if err != nil || (format != "jpeg" && format != "png") || config.Width < 1 || config.Height < 1 || config.Width > 4000 || config.Height > 4000 || int64(config.Width)*int64(config.Height) > 12000000 {
		fail(w, 400, "Image must be a JPEG or PNG up to 12 megapixels")
		return "", "", 0, false
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		unavailable(w, r, err)
		return "", "", 0, false
	}
	img, format, err := image.Decode(file)
	if err != nil {
		fail(w, 400, "Invalid image")
		return "", "", 0, false
	}
	ext := "jpg"
	if format == "png" {
		ext = "png"
	}
	dir := filepath.Dir(imagePath("placeholder", ext))
	if err = os.MkdirAll(dir, 0700); err != nil {
		unavailable(w, r, err)
		return "", "", 0, false
	}
	temp, err := os.CreateTemp(dir, "upload-*")
	if err != nil {
		unavailable(w, r, err)
		return "", "", 0, false
	}
	tempName := temp.Name()
	if ext == "jpg" {
		err = jpeg.Encode(temp, img, &jpeg.Options{Quality: 82})
	} else {
		err = png.Encode(temp, img)
	}
	if err != nil {
		temp.Close()
		os.Remove(tempName)
		unavailable(w, r, err)
		return "", "", 0, false
	}
	info, err := temp.Stat()
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(tempName)
		unavailable(w, r, err)
		return "", "", 0, false
	}
	if info.Size() > 6<<20 {
		os.Remove(tempName)
		fail(w, 400, "Encoded image is too large")
		return "", "", 0, false
	}
	return tempName, ext, info.Size(), true
}

func (a *App) uploadBusinessLogo(w http.ResponseWriter, r *http.Request) {
	tempName, ext, size, ok := receiveImage(w, r)
	if !ok {
		return
	}
	defer os.Remove(tempName)
	newImageID := newID()
	target := imagePath(newImageID, ext)
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		unavailable(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	var plan string
	var oldID, oldExt *string
	err = tx.QueryRow(r.Context(), `SELECT CASE WHEN plan_expires_at>now() THEN plan ELSE 'free' END,logo_id::text,logo_ext FROM businesses WHERE id=$1 FOR UPDATE`, who(r).BusinessID).Scan(&plan, &oldID, &oldExt)
	if err != nil {
		unavailable(w, r, err)
		return
	}
	var productUsed, retainedLogoUsed int64
	if err = tx.QueryRow(r.Context(), `SELECT COALESCE(sum(image_size),0) FROM products WHERE business_id=$1`, who(r).BusinessID).Scan(&productUsed); err != nil {
		unavailable(w, r, err)
		return
	}
	if err = tx.QueryRow(r.Context(), `SELECT COALESCE(sum(logo_size),0) FROM (SELECT max(logo_size) logo_size FROM invoices WHERE business_id=$1 AND logo_id IS NOT NULL GROUP BY logo_id) retained`, who(r).BusinessID).Scan(&retainedLogoUsed); err != nil {
		unavailable(w, r, err)
		return
	}
	if productUsed+retainedLogoUsed+size > planImageLimit(plan) {
		fail(w, 403, "Image storage allowance reached")
		return
	}
	if err = os.Rename(tempName, target); err != nil {
		unavailable(w, r, err)
		return
	}
	if _, err = tx.Exec(r.Context(), `UPDATE businesses SET logo_id=$1,logo_ext=$2,logo_size=$3 WHERE id=$4`, newImageID, ext, size, who(r).BusinessID); err != nil {
		os.Remove(target)
		unavailable(w, r, err)
		return
	}
	var oldLogoUsed bool
	if oldID != nil {
		if err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM invoices WHERE business_id=$1 AND logo_id=$2)`, who(r).BusinessID, *oldID).Scan(&oldLogoUsed); err != nil {
			os.Remove(target)
			unavailable(w, r, err)
			return
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		os.Remove(target)
		unavailable(w, r, err)
		return
	}
	if oldID != nil && oldExt != nil && !oldLogoUsed {
		_ = os.Remove(imagePath(*oldID, *oldExt))
	}
	writeJSON(w, 200, map[string]any{"has_logo": true, "logo_version": newImageID})
}

func (a *App) deleteBusinessLogo(w http.ResponseWriter, r *http.Request) {
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		unavailable(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	var oldID, oldExt *string
	if err = tx.QueryRow(r.Context(), `SELECT logo_id::text,logo_ext FROM businesses WHERE id=$1 FOR UPDATE`, who(r).BusinessID).Scan(&oldID, &oldExt); err != nil {
		unavailable(w, r, err)
		return
	}
	if _, err = tx.Exec(r.Context(), `UPDATE businesses SET logo_id=NULL,logo_ext=NULL,logo_size=0 WHERE id=$1`, who(r).BusinessID); err != nil {
		unavailable(w, r, err)
		return
	}
	var oldLogoUsed bool
	if oldID != nil {
		if err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM invoices WHERE business_id=$1 AND logo_id=$2)`, who(r).BusinessID, *oldID).Scan(&oldLogoUsed); err != nil {
			unavailable(w, r, err)
			return
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		unavailable(w, r, err)
		return
	}
	if oldID != nil && oldExt != nil && !oldLogoUsed {
		_ = os.Remove(imagePath(*oldID, *oldExt))
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) privateBusinessLogo(w http.ResponseWriter, r *http.Request) {
	var imageID, ext *string
	err := a.DB.QueryRow(r.Context(), `SELECT logo_id::text,logo_ext FROM businesses WHERE id=$1`, who(r).BusinessID).Scan(&imageID, &ext)
	a.serveBusinessLogo(w, r, imageID, ext, err, false)
}

func (a *App) publicBusinessLogo(w http.ResponseWriter, r *http.Request) {
	var imageID, ext *string
	err := a.DB.QueryRow(r.Context(), `SELECT logo_id::text,logo_ext FROM businesses WHERE slug=$1 AND published=true`, chi.URLParam(r, "slug")).Scan(&imageID, &ext)
	a.serveBusinessLogo(w, r, imageID, ext, err, true)
}

func (a *App) privateInvoiceLogo(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !validID(id) {
		fail(w, 404, "Logo not found")
		return
	}
	var imageID, ext *string
	err := a.DB.QueryRow(r.Context(), `SELECT logo_id::text,logo_ext FROM invoices WHERE id=$1 AND business_id=$2 AND show_logo`, id, who(r).BusinessID).Scan(&imageID, &ext)
	a.serveBusinessLogo(w, r, imageID, ext, err, false)
}

func (a *App) publicInvoiceLogo(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if len(token) != 64 {
		fail(w, 404, "Logo not found")
		return
	}
	var imageID, ext *string
	err := a.DB.QueryRow(r.Context(), `SELECT logo_id::text,logo_ext FROM invoices WHERE public_token_hash=$1 AND status='sent' AND show_logo`, hash(token)).Scan(&imageID, &ext)
	a.serveBusinessLogo(w, r, imageID, ext, err, true)
}

func (a *App) serveBusinessLogo(w http.ResponseWriter, r *http.Request, imageID, ext *string, err error, public bool) {
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (imageID == nil || ext == nil)) {
		fail(w, 404, "Logo not found")
		return
	}
	if err != nil {
		unavailable(w, r, err)
		return
	}
	serveStoredImage(w, r, *imageID, *ext, public)
}

func (a *App) privateProductImage(w http.ResponseWriter, r *http.Request) {
	a.serveProductImage(w, r, false)
}
func (a *App) publicProductImage(w http.ResponseWriter, r *http.Request) {
	a.serveProductImage(w, r, true)
}
func (a *App) serveProductImage(w http.ResponseWriter, r *http.Request, public bool) {
	id := chi.URLParam(r, "id")
	if !validID(id) {
		fail(w, 404, "Image not found")
		return
	}
	query := `SELECT p.image_id::text,p.image_ext FROM products p WHERE p.id=$1 AND p.business_id=$2`
	args := []any{id}
	if public {
		query = `SELECT p.image_id::text,p.image_ext FROM products p JOIN businesses b ON b.id=p.business_id WHERE p.id=$1 AND p.active AND b.published`
	} else {
		args = append(args, who(r).BusinessID)
	}
	var imageID, ext *string
	err := a.DB.QueryRow(r.Context(), query, args...).Scan(&imageID, &ext)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "Image not found")
		return
	}
	if err != nil {
		unavailable(w, r, err)
		return
	}
	if imageID == nil || ext == nil {
		fail(w, 404, "Image not found")
		return
	}
	serveStoredImage(w, r, *imageID, *ext, public)
}

func serveStoredImage(w http.ResponseWriter, r *http.Request, imageID, ext string, public bool) {
	if !validID(imageID) || (ext != "jpg" && ext != "png") {
		fail(w, 404, "Image not found")
		return
	}
	file, err := os.Open(imagePath(imageID, ext))
	if errors.Is(err, os.ErrNotExist) {
		fail(w, 404, "Image not found")
		return
	}
	if err != nil {
		unavailable(w, r, err)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		unavailable(w, r, err)
		return
	}
	if ext == "jpg" {
		w.Header().Set("Content-Type", "image/jpeg")
	} else {
		w.Header().Set("Content-Type", "image/png")
	}
	if public {
		w.Header().Set("Cache-Control", "public, max-age=300")
	} else {
		w.Header().Set("Cache-Control", "private, max-age=300")
	}
	http.ServeContent(w, r, strings.TrimSuffix(info.Name(), "."+ext), info.ModTime(), file)
}
