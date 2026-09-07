package gamespy

import (
	"errors"
	"owfc/common"
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

		err := common.PutReflectString(v, value)
		if err != nil {
			return out, err
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
	for field, v := range v.Fields() {
		split := strings.Split(field.Tag.Get("gs"), ",")
		key := split[0]
		if key == "" {
			continue
		}

		if slices.Contains(split, "omitzero") && v.IsZero() {
			continue
		}

		out.WriteByte('\\')
		out.WriteString(key)
		out.WriteByte('\\')

		if v.Kind() == reflect.Slice {
			for _, v := range v.Fields() {
				marshalStruct(out, v)
			}

			continue
		}

		switch v.Kind() {
		case reflect.String:
			out.WriteString(v.String())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			out.WriteString(strconv.FormatUint(v.Uint(), 10))
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			out.WriteString(strconv.FormatInt(v.Int(), 10))
		case reflect.Bool:
			str := "0"
			if v.Bool() {
				str = "1"
			}

			out.WriteString(str)
		}
	}
}
