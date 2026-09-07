package nas

import (
	"fmt"
	"net/url"
	"owfc/common"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func Unmarshal[T any](param url.Values) (T, error) {
	var out T

	unmarshalStruct(reflect.ValueOf(&out).Elem(), param)

	return out, nil
}

func unmarshalStruct(v reflect.Value, param url.Values) error {
	for field, v := range v.Fields() {
		if field.Anonymous && v.Kind() == reflect.Struct {
			unmarshalStruct(v, param)
			continue
		}

		key := field.Tag.Get("nas")
		value, err := common.Base64DwcEncoding.DecodeString(param.Get(key))
		if err != nil || len(value) == 0 {
			continue
		}

		err = common.PutReflectString(v, string(value))
		if err != nil {
			return err
		}
	}

	return nil
}

func Marshal(T any) string {
	param := url.Values{}

	marshalStruct(&param, reflect.ValueOf(T))

	return strings.ReplaceAll(param.Encode(), "%2A", "*") + "\x00"
}

func marshalStruct(param *url.Values, v reflect.Value) {
	for field, v := range v.Fields() {
		if field.Anonymous && v.Kind() == reflect.Struct {
			marshalStruct(param, v)
			continue
		}

		key := field.Tag.Get("nas")
		if key == "" {
			continue
		}

		var s string
		switch v.Kind() {
		case reflect.String:
			s = v.String()
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			s = strconv.FormatUint(v.Uint(), 10)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			s = strconv.FormatInt(v.Int(), 10)
		case reflect.Bool:
			str := "0"
			if v.Bool() {
				str = "1"
			}

			s = str
		}
		param.Set(key, common.Base64DwcEncoding.EncodeToString([]byte(s)))
	}
}

func getDateTime() string {
	t := time.Now().UTC()
	return fmt.Sprintf("%04d%02d%02d%02d%02d%02d", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
}
