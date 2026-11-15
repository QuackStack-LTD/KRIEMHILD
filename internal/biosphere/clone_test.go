package biosphere

import (
	"encoding/json"
	"reflect"
	"testing"
)

// Populate every reference field, including ones added to the model later.
func populateCloneFields(v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer:
		v.Set(reflect.New(v.Type().Elem()))
		populateCloneFields(v.Elem())
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			populateCloneFields(v.Field(i))
		}
	case reflect.Slice:
		v.Set(reflect.MakeSlice(v.Type(), 1, 1))
		populateCloneFields(v.Index(0))
	case reflect.Map:
		v.Set(reflect.MakeMap(v.Type()))
		key, value := reflect.New(v.Type().Key()).Elem(), reflect.New(v.Type().Elem()).Elem()
		populateCloneFields(key)
		populateCloneFields(value)
		v.SetMapIndex(key, value)
	case reflect.String:
		v.SetString("source")
	case reflect.Int:
		v.SetInt(1)
	case reflect.Float64:
		v.SetFloat(1)
	case reflect.Bool:
		v.SetBool(true)
	default:
		panic("unhandled clone test field: " + v.Kind().String())
	}
}

func mutateCloneFields(v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer:
		mutateCloneFields(v.Elem())
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			mutateCloneFields(v.Field(i))
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			mutateCloneFields(v.Index(i))
		}
	case reflect.Map:
		for _, key := range v.MapKeys() {
			value := reflect.New(v.Type().Elem()).Elem()
			value.Set(v.MapIndex(key))
			mutateCloneFields(value)
			v.SetMapIndex(key, value)
		}
	case reflect.String:
		v.SetString("changed")
	case reflect.Int:
		v.SetInt(2)
	case reflect.Float64:
		v.SetFloat(2)
	case reflect.Bool:
		v.SetBool(false)
	}
}

func TestTypedBiosphereClonesPreserveEveryFieldAndIndependentReferences(t *testing.T) {
	var source State
	populateCloneFields(reflect.ValueOf(&source).Elem())
	before, _ := json.Marshal(source)
	copy := CloneState(source)
	if !reflect.DeepEqual(source, copy) {
		t.Fatal("state clone lost fields")
	}
	mutateCloneFields(reflect.ValueOf(&copy).Elem())
	after, _ := json.Marshal(source)
	if string(before) != string(after) {
		t.Fatal("state clone aliases source")
	}
	var lifeform Lifeform
	populateCloneFields(reflect.ValueOf(&lifeform).Elem())
	before, _ = json.Marshal(lifeform)
	cloned := CloneLifeform(&lifeform)
	if !reflect.DeepEqual(lifeform, *cloned) {
		t.Fatal("lifeform clone lost fields")
	}
	mutateCloneFields(reflect.ValueOf(cloned).Elem())
	after, _ = json.Marshal(lifeform)
	if string(before) != string(after) {
		t.Fatal("lifeform clone aliases source")
	}
	if CloneLifeform(nil) != nil {
		t.Fatal("nil biological profile changed")
	}
}
