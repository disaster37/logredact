package logredact

import (
	"reflect"
	"regexp"

	"github.com/sirupsen/logrus"
)

type LogRedact struct {
	secrets  []*regexp.Regexp
	replacer string
}

func New(secrets []string, replacer string) *LogRedact {

	var compiledSecrets []*regexp.Regexp
	for _, secret := range secrets {
		compiledSecrets = append(compiledSecrets, regexp.MustCompile(secret))
	}

	return &LogRedact{secrets: compiledSecrets, replacer: replacer}
}

func (h *LogRedact) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *LogRedact) Fire(entry *logrus.Entry) error {
	entry.Message = h.replaceSecrets(entry.Message)

	for key, value := range entry.Data {
		entry.Data[key] = h.processValue(reflect.ValueOf(value))
	}
	return nil
}

func (h *LogRedact) processValue(v reflect.Value) interface{} {
	if !v.IsValid() {
		return nil
	}

	switch v.Kind() {
	case reflect.String:
		return h.replaceSecrets(v.String())

	case reflect.Ptr:
		if v.IsNil() {
			return nil
		}
		elem := v.Elem()
		newElem := reflect.New(elem.Type())
		h.processValueRecursively(elem, newElem.Elem())
		return newElem.Interface()

	case reflect.Struct:
		newStruct := reflect.New(v.Type()).Elem()
		h.processValueRecursively(v, newStruct)
		return newStruct.Interface()

	case reflect.Slice:
		newSlice := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := 0; i < v.Len(); i++ {
			newSlice.Index(i).Set(reflect.ValueOf(h.processValue(v.Index(i))))
		}
		return newSlice.Interface()

	case reflect.Map:
		newMap := reflect.MakeMap(v.Type())
		iter := v.MapRange()
		for iter.Next() {
			k := iter.Key()
			v := iter.Value()
			newV := h.processValue(v)
			newMap.SetMapIndex(k, reflect.ValueOf(newV))
		}
		return newMap.Interface()
	}

	return v.Interface()
}

func (h *LogRedact) processValueRecursively(src, dest reflect.Value) {
	for i := 0; i < src.NumField(); i++ {
		dest.Field(i).Set(reflect.ValueOf(h.processValue(src.Field(i))))
	}
}

func (h *LogRedact) replaceSecrets(s string) string {
	for _, secret := range h.secrets {
		s = secret.ReplaceAllString(s, h.replacer)
	}
	return s
}
