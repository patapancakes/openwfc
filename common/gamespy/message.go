package gamespy

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

var (
	ErrInvalidGameSpyCommand = errors.New("invalid GameSpy command received")
)

const EndDelimiter = `\final\`

func Unmarshal[T any](msg string) (T, error) {
	var out T

	var found bool
	msg, found = strings.CutSuffix(msg, EndDelimiter)
	if !found {
		return out, ErrInvalidGameSpyCommand
	}

	kvs := KeyValuesFromString(msg)

	for field, v := range reflect.ValueOf(&out).Elem().Fields() {
		split := strings.Split(field.Tag.Get("gs"), ",")
		key := split[0]

		value := kvs.Get(key)
		if slices.Contains(split, "raw") {
			value, _, _ = strings.Cut(msg, `\`+key+`\`)
		}
		if value == "" {
			continue
		}

		switch v.Kind() {
		case reflect.String:
			v.SetString(value)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			n, err := strconv.ParseUint(value, 10, v.Type().Bits())
			if err != nil {
				return out, fmt.Errorf("%s: %w", key, err)
			}

			v.SetUint(n)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			n, err := strconv.ParseInt(value, 10, v.Type().Bits())
			if err != nil {
				return out, fmt.Errorf("%s: %w", key, err)
			}

			v.SetInt(n)
		case reflect.Bool:
			b, err := strconv.ParseBool(value)
			if err != nil {
				return out, fmt.Errorf("%s: %w", key, err)
			}

			v.SetBool(b)
		default:
			return out, fmt.Errorf("unsupported field type %s for %q", v.Type(), key)
		}
	}

	return out, nil
}

func Marshal(T any) string {
	// return empty string if NoResponse
	_, noResp := T.(NoResponse)
	if noResp {
		return ""
	}

	// otherwise build response
	var out strings.Builder

	marshalStruct(&out, reflect.ValueOf(T))

	out.WriteString(EndDelimiter)
	return out.String()
}

func marshalStruct(out *strings.Builder, v reflect.Value) {
	for field, val := range v.Fields() {
		split := strings.Split(field.Tag.Get("gs"), ",")
		key := split[0]
		if key == "" {
			continue
		}

		if slices.Contains(split, "omitzero") && val.IsZero() {
			continue
		}

		out.WriteByte('\\')
		out.WriteString(key)
		out.WriteByte('\\')

		if val.Kind() == reflect.Slice {
			for _, v := range val.Fields() {
				marshalStruct(out, v)
			}

			continue
		}

		switch val.Kind() {
		case reflect.String:
			out.WriteString(val.String())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			out.WriteString(strconv.FormatUint(val.Uint(), 10))
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			out.WriteString(strconv.FormatInt(val.Int(), 10))
		case reflect.Bool:
			str := "0"
			if val.Bool() {
				str = "1"
			}

			out.WriteString(str)
		}
	}
}
