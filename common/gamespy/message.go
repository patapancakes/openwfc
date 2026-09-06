package gamespy

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

var (
	ErrInvalidGameSpyCommand = errors.New("invalid GameSpy command received")
)

func Unmarshal[T any](msg string) (T, error) {
	var out T

	v := reflect.ValueOf(&out).Elem()
	t := v.Type()

	var found bool
	msg, found = strings.CutSuffix(msg, `\final\`)
	if !found {
		return out, ErrInvalidGameSpyCommand
	}

	kvs := KeyValuesFromString(msg)

	for field := range t.Fields() {
		key := field.Tag.Get("gs")
		value := kvs.Get(key)
		if value == "" {
			continue
		}

		switch field.Type.Kind() {
		case reflect.String:
			v.FieldByIndex(field.Index).SetString(value)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			n, err := strconv.ParseUint(value, 10, field.Type.Bits())
			if err != nil {
				return out, fmt.Errorf("%s: %w", key, err)
			}

			v.FieldByIndex(field.Index).SetUint(n)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			n, err := strconv.ParseInt(value, 10, field.Type.Bits())
			if err != nil {
				return out, fmt.Errorf("%s: %w", key, err)
			}

			v.FieldByIndex(field.Index).SetInt(n)
		case reflect.Bool:
			b, err := strconv.ParseBool(value)
			if err != nil {
				return out, fmt.Errorf("%s: %w", key, err)
			}

			v.FieldByIndex(field.Index).SetBool(b)
		default:
			return out, fmt.Errorf("unsupported field type %s for %q", field.Type, key)
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

	v := reflect.ValueOf(T)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	for field := range v.Fields() {
		key := field.Tag.Get("gs")
		if key == "" {
			continue
		}

		out.WriteString(`\`)
		out.WriteString(key)
		out.WriteString(`\`)

		val := v.FieldByIndex(field.Index)
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

	out.WriteString(`\final\`)

	return out.String()
}
