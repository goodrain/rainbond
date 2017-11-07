package govalidator

import (
	"reflect"
	"strings"
)

// ROADMAP
// traverse map or struct
// detect each type
// if type is struct or map then traverse it
// if type is not struct or map then just push them in parent map's key as key and value of it
// make flatten all the type in map[string]interface{}
// in this case mapWalker will do the task

// roller represents a roller type that will be used to flatten our data in a map[string]interface{}
type roller struct {
	root          map[string]interface{}
	typeName      string
	tagIdentifier string
	tagSeparator  string
}

// start start traversing through the tree
func (r *roller) start(iface interface{}) {
	//initialize the Tree
	r.root = make(map[string]interface{}, 0)
	r.typeName = ""
	ifv := reflect.ValueOf(iface)
	ift := reflect.TypeOf(iface)
	if ift.Kind() == reflect.Ptr {
		ifv = ifv.Elem()
		ift = ift.Elem()
	}
	canInterface := ifv.CanInterface()
	//check the provided root elment
	switch ift.Kind() {
	case reflect.Struct:
		if canInterface {
			r.traverseStruct(ifv.Interface())
		}
	case reflect.Map:
		if ifv.Len() > 0 {
			if canInterface {
				r.traverseMap(ifv.Interface())
			}
		}
	case reflect.Slice:
		if canInterface {
			r.push("slice", ifv.Interface())
		}
	}
}

// setTagIdentifier set the struct tag identifier. e.g: json, validate etc
func (r *roller) setTagIdentifier(i string) {
	r.tagIdentifier = i
}

// setTagSeparator set the struct tag separator. e.g: pipe (|) or comma (,)
func (r *roller) setTagSeparator(s string) {
	r.tagSeparator = s
}

// getFlatMap get the all flatten values
func (r *roller) getFlatMap() map[string]interface{} {
	return r.root
}

// getFlatVal return interfac{} value if exist
func (r *roller) getFlatVal(key string) (interface{}, bool) {
	var val interface{}
	var ok bool
	if val, ok = r.root[key]; ok {
		return val, ok
	}
	return val, ok
}

// push add value to map if key does not exist
func (r *roller) push(key string, val interface{}) bool {
	if _, ok := r.root[key]; ok {
		return false
	}
	r.root[key] = val
	return true
}

// traverseStruct through all structs and add it to root
func (r *roller) traverseStruct(iface interface{}) {
	ifv := reflect.ValueOf(iface)
	ift := reflect.TypeOf(iface)

	if ift.Kind() == reflect.Ptr {
		ifv = ifv.Elem()
		ift = ift.Elem()
	}

	for i := 0; i < ift.NumField(); i++ {
		v := ifv.Field(i)
		rfv := ift.Field(i)

		switch v.Kind() {
		case reflect.Struct:
			if v.CanInterface() {
				r.typeName = ift.Name()
				r.traverseStruct(v.Interface())
			}
		case reflect.Map:
			if v.CanInterface() {
				r.traverseMap(v.Interface())
			}
		case reflect.Ptr: // if the field inside struct is Ptr then get the type and underlying values as interface{}
			ptrReflectionVal := reflect.Indirect(v)
			if !isEmpty(ptrReflectionVal) {
				ptrField := ptrReflectionVal.Type()
				switch ptrField.Kind() {
				case reflect.Struct:
					if v.CanInterface() {
						r.traverseStruct(v.Interface())
					}
				case reflect.Map:
					if v.CanInterface() {
						r.traverseMap(v.Interface())
					}
				}
			}
		default:
			if len(rfv.Tag.Get(r.tagIdentifier)) > 0 {
				tags := strings.Split(rfv.Tag.Get(r.tagIdentifier), r.tagSeparator)
				// add if first tag is not hyphen
				if tags[0] != "-" {
					if v.CanInterface() {
						r.push(tags[0], v.Interface())
					}
				}
			} else {
				if v.Kind() == reflect.Ptr {
					if ifv.CanInterface() {
						r.push(ift.Name()+"."+rfv.Name, ifv.Interface())
					}
				} else {
					if v.CanInterface() {
						r.push(ift.Name()+"."+rfv.Name, v.Interface())
					}
				}
			}
		}
	}
}

// traverseMap through all the map and add it to root
func (r *roller) traverseMap(iface interface{}) {
	ifv := reflect.ValueOf(iface)
	ift := reflect.TypeOf(iface)
	if ift.Kind() == reflect.Ptr {
		ifv = ifv.Elem()
		ift = ift.Elem()
	}

	switch iface.(type) {
	case map[string]interface{}:
		for k, v := range iface.(map[string]interface{}) {
			switch reflect.TypeOf(v).Kind() {
			case reflect.Struct:
				r.typeName = k // set the map key as name
				r.traverseStruct(v)
			case reflect.Map:
				r.typeName = k // set the map key as name
				r.traverseMap(v)
			case reflect.Ptr: // if the field inside map is Ptr then get the type and underlying values as interface{}
				switch reflect.TypeOf(v).Elem().Kind() {
				case reflect.Struct:
					r.traverseStruct(v)
				case reflect.Map:
					switch v.(type) {
					case *map[string]interface{}:
						r.traverseMap(*v.(*map[string]interface{}))
					case *map[string]string:
						r.traverseMap(*v.(*map[string]string))
					case *map[string]bool:
						r.traverseMap(*v.(*map[string]bool))
					case *map[string]int:
						r.traverseMap(*v.(*map[string]int))
					case *map[string]int8:
						r.traverseMap(*v.(*map[string]int8))
					case *map[string]int16:
						r.traverseMap(*v.(*map[string]int16))
					case *map[string]int32:
						r.traverseMap(*v.(*map[string]int32))
					case *map[string]int64:
						r.traverseMap(*v.(*map[string]int64))
					case *map[string]float32:
						r.traverseMap(*v.(*map[string]float32))
					case *map[string]float64:
						r.traverseMap(*v.(*map[string]float64))
					case *map[string]uint:
						r.traverseMap(*v.(*map[string]uint))
					case *map[string]uint8:
						r.traverseMap(*v.(*map[string]uint8))
					case *map[string]uint16:
						r.traverseMap(*v.(*map[string]uint16))
					case *map[string]uint32:
						r.traverseMap(*v.(*map[string]uint32))
					case *map[string]uint64:
						r.traverseMap(*v.(*map[string]uint64))
					case *map[string]uintptr:
						r.traverseMap(*v.(*map[string]uintptr))
					}
				default:
					r.push(k, v.(interface{}))
				}
			default:
				r.push(k, v)
			}
		}
	case map[string]string:
		for k, v := range iface.(map[string]string) {
			r.push(k, v)
		}
	case map[string]bool:
		for k, v := range iface.(map[string]bool) {
			r.push(k, v)
		}
	case map[string]int:
		for k, v := range iface.(map[string]int) {
			r.push(k, v)
		}
	case map[string]int8:
		for k, v := range iface.(map[string]int8) {
			r.push(k, v)
		}
	case map[string]int16:
		for k, v := range iface.(map[string]int16) {
			r.push(k, v)
		}
	case map[string]int32:
		for k, v := range iface.(map[string]int32) {
			r.push(k, v)
		}
	case map[string]int64:
		for k, v := range iface.(map[string]int64) {
			r.push(k, v)
		}
	case map[string]float32:
		for k, v := range iface.(map[string]float32) {
			r.push(k, v)
		}
	case map[string]float64:
		for k, v := range iface.(map[string]float64) {
			r.push(k, v)
		}
	case map[string]uint:
		for k, v := range iface.(map[string]uint) {
			r.push(k, v)
		}
	case map[string]uint8:
		for k, v := range iface.(map[string]uint8) {
			r.push(k, v)
		}
	case map[string]uint16:
		for k, v := range iface.(map[string]uint16) {
			r.push(k, v)
		}
	case map[string]uint32:
		for k, v := range iface.(map[string]uint32) {
			r.push(k, v)
		}
	case map[string]uint64:
		for k, v := range iface.(map[string]uint64) {
			r.push(k, v)
		}
	case map[string]uintptr:
		for k, v := range iface.(map[string]uintptr) {
			r.push(k, v)
		}
	}
}
