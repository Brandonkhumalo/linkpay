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
	r.Body = http.MaxBytesReader(w, r.Body, maxImageBytes+65536)
	if err := r.ParseMultipartForm(maxImageBytes); err != nil {
		fail(w, 400, "Image must be under 3 MB")
		return
	}
	file, header, err := r.FormFile("image")
	if err != nil || header.Size > maxImageBytes {
		fail(w, 400, "Choose a JPEG or PNG under 3 MB")
		return
	}
	defer file.Close()
	config, format, err := image.DecodeConfig(file)
	if err != nil || (format != "jpeg" && format != "png") || config.Width < 1 || config.Height < 1 || config.Width > 4000 || config.Height > 4000 || int64(config.Width)*int64(config.Height) > 12000000 {
		fail(w, 400, "Image must be a JPEG or PNG up to 12 megapixels")
		return
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		unavailable(w, r, err)
		return
	}
	img, format, err := image.Decode(file)
	if err != nil {
		fail(w, 400, "Invalid image")
		return
	}
	ext := "jpg"
	if format == "png" {
		ext = "png"
	}
	dir := filepath.Dir(imagePath("placeholder", ext))
	if err = os.MkdirAll(dir, 0700); err != nil {
		unavailable(w, r, err)
		return
	}
	temp, err := os.CreateTemp(dir, "upload-*")
	if err != nil {
		unavailable(w, r, err)
		return
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if ext == "jpg" {
		err = jpeg.Encode(temp, img, &jpeg.Options{Quality: 82})
	} else {
		err = png.Encode(temp, img)
	}
	if err != nil {
		temp.Close()
		unavailable(w, r, err)
		return
	}
	info, err := temp.Stat()
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		unavailable(w, r, err)
		return
	}
	if info.Size() > 6<<20 {
		fail(w, 400, "Encoded image is too large")
		return
	}
	newImageID := newID()
	target := imagePath(newImageID, ext)
	tx, err := a.DB.Begin(r.Context())
	if err != nil {
		unavailable(w, r, err)
		return
	}
	defer tx.Rollback(r.Context())
	var plan string
	if err = tx.QueryRow(r.Context(), `SELECT CASE WHEN plan_expires_at>now() THEN plan ELSE 'free' END FROM businesses WHERE id=$1 FOR UPDATE`, who(r).BusinessID).Scan(&plan); err != nil {
		unavailable(w, r, err)
		return
	}
	var oldID, oldExt *string
	var oldSize int64
	err = tx.QueryRow(r.Context(), `SELECT image_id::text,image_ext,image_size FROM products WHERE id=$1 AND business_id=$2 FOR UPDATE`, id, who(r).BusinessID).Scan(&oldID, &oldExt, &oldSize)
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
	if used-oldSize+info.Size() > planImageLimit(plan) {
		fail(w, 403, "Image storage allowance reached")
		return
	}
	if err = os.Rename(tempName, target); err != nil {
		unavailable(w, r, err)
		return
	}
	if _, err = tx.Exec(r.Context(), `UPDATE products SET image_id=$1,image_ext=$2,image_size=$3 WHERE id=$4 AND business_id=$5`, newImageID, ext, info.Size(), id, who(r).BusinessID); err != nil {
		os.Remove(target)
		unavailable(w, r, err)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		os.Remove(target)
		unavailable(w, r, err)
		return
	}
	if oldID != nil && oldExt != nil {
		_ = os.Remove(imagePath(*oldID, *oldExt))
	}
	writeJSON(w, 200, map[string]any{"has_image": true})
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
	if !validID(*imageID) || (*ext != "jpg" && *ext != "png") {
		fail(w, 404, "Image not found")
		return
	}
	file, err := os.Open(imagePath(*imageID, *ext))
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
	if *ext == "jpg" {
		w.Header().Set("Content-Type", "image/jpeg")
	} else {
		w.Header().Set("Content-Type", "image/png")
	}
	w.Header().Set("Cache-Control", "private, max-age=300")
	if public {
		w.Header().Set("Cache-Control", "public, max-age=300")
	}
	http.ServeContent(w, r, strings.TrimSuffix(info.Name(), "."+*ext), info.ModTime(), file)
}
