package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
)

const maxBodyBytes = 1 << 20 // 1 MB is far above any real polygon; stops memory abuse

// decodeJSON reads exactly one JSON object from the body into dst.
// The returned error's message is safe and useful to show the client.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	decoder := json.NewDecoder(r.Body)

	if err := decoder.Decode(dst); err != nil {
		return describeDecodeError(err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}

func describeDecodeError(err error) error {
	var (
		syntaxErr   *json.SyntaxError
		typeErr     *json.UnmarshalTypeError
		tooLargeErr *http.MaxBytesError
	)
	switch {
	case errors.Is(err, io.EOF):
		return errors.New("request body is empty; expected a JSON object")
	case errors.Is(err, io.ErrUnexpectedEOF):
		return errors.New("request body is not valid JSON (it ends too early)")
	case errors.As(err, &syntaxErr):
		return fmt.Errorf("request body is not valid JSON (error at byte %d)", syntaxErr.Offset)
	case errors.As(err, &typeErr) && typeErr.Field == "":
		return errors.New("request body must be a JSON object")
	case errors.As(err, &typeErr):
		return fmt.Errorf("field %q must be %s, got %s", typeErr.Field, jsonTypeName(typeErr.Type), typeErr.Value)
	case errors.As(err, &tooLargeErr):
		return fmt.Errorf("request body must be at most %d bytes", tooLargeErr.Limit)
	default:
		return errors.New("request body could not be read")
	}
}

// jsonTypeName describes a Go type the way a JSON client thinks about it.
func jsonTypeName(t reflect.Type) string {
	switch t.Kind() {
	case reflect.String:
		return "a string"
	case reflect.Bool:
		return "a boolean"
	case reflect.Int, reflect.Int64, reflect.Float32, reflect.Float64:
		return "a number"
	case reflect.Slice, reflect.Array:
		return "an array"
	default:
		return "an object"
	}
}
