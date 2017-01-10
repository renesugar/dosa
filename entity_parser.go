// Copyright (c) 2017 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package dosa

import (
	"fmt"
	"reflect"
	"strings"
	"unicode"
)

func parsePrimaryKey(tableName string, s string) (*keyDeclaration, error) {
	k := keyDeclaration{}

	s = removeSpaces(s)
	// remove set of matching open and close parens over whole string
	if strings.HasSuffix(s, ")") && strings.HasPrefix(s, "(") {
		s = s[1 : len(s)-1]
	}

	// look for multi-component partition key notation
	if strings.HasPrefix(s, "(") {
		closeIndex := strings.Index(s, ")")
		// complex case: (a,b),c,d
		k.partitionKeys = strings.Split(s[1:closeIndex], ",")
		if closeIndex < len(s)-1 {
			if s[closeIndex+1] != ',' {
				return nil, fmt.Errorf("Object %q missing comma after partition key close parenthesis", tableName)
			}
			k.primaryKeys = strings.Split(s[closeIndex+2:], ",")
		} else {
			k.primaryKeys = nil
		}
	} else {
		// not using multi-component partition key syntax, so first element
		// is the partition key, remaining elements are primary keys
		// simple case: a,b,c
		fields := strings.Split(s, ",")

		k.partitionKeys = []string{fields[0]}
		k.primaryKeys = fields[1:]
	}

	// search for duplicates
	everything := append(k.primaryKeys, k.partitionKeys...)
	seen := map[string]bool{}
	for v := range everything {
		if seen[everything[v]] {
			return nil, fmt.Errorf("Object %q has duplicate field %q in key struct tag", tableName, everything[v])
		}
		if everything[v] == "" {
			return nil, fmt.Errorf("Object %q has an empty primaryKey column", tableName)
		}

		seen[everything[v]] = true
	}

	return &k, nil
}

// TableFromInstance creates a dosa.Table from an instance
func TableFromInstance(object interface{}) (*Table, error) {
	value := reflect.ValueOf(object)
	if value.Type().Kind() != reflect.Ptr {
		return nil, fmt.Errorf("Passed type %q, expected a pointer to a dosa-annotated struct (did you forget an ampersand?)", value.Type().Kind())
	}
	elem := value.Elem()
	d := Table{}
	d.StructName = elem.Type().Name()
	d.TableName = d.StructName
	d.Types = map[string]Type{}
	for i := 0; i < elem.NumField(); i++ {
		structField := elem.Type().Field(i)
		name := structField.Name
		dosaAnnotation := structField.Tag.Get("dosa")
		if name == "Entity" {
			if dosaAnnotation == "" {
				return nil, fmt.Errorf("dosa.Entity on object %s found without a dosa struct tag", d.StructName)
			}
			attrs := strings.Split(dosaAnnotation, ",")
			var saved string
			for _, attr := range attrs {
				if saved != "" {
					attr = saved + "," + attr
					saved = ""
				}
				if strings.HasPrefix(attr, "primaryKey=") {
					// could hardcode the offset, but this is cleaner
					pkString := strings.SplitN(attr, "=", 2)[1]
					var err error
					// TODO: this could be better
					if strings.Count(pkString, "(") > strings.Count(pkString, ")") {
						saved = attr
						continue
					}

					d.Keys, err = parsePrimaryKey(d.StructName, pkString)
					if err != nil {
						return nil, err
					}
				} else {
					return nil, fmt.Errorf("Invalid annotation %q found on object %q", attr, d.StructName)
				}
			}
			if saved != "" {
				return nil, fmt.Errorf("Object %q missing close parenthesis for primary key struct tag", d.StructName)
			}
		} else {
			typ, err := typify(structField.Type)
			if err != nil {
				return nil, err
			}
			d.Types[name] = typ
		}
	}

	/* TODO: Check that all the fields exist
	everything := append(d.Keys.primaryKeys, d.Keys..partitionKeys...)
	for _, value := range everything {
		...
	}
	*/

	return &d, nil
}

func typify(f reflect.Type) (Type, error) {
	switch f.Kind() {
	case reflect.Bool:
		return Bool, nil
	case reflect.Int32:
		return Int32, nil
	case reflect.Int64:
		return Int64, nil
	case reflect.Float64:
		return Double, nil
	case reflect.String:
		return String, nil
	// TODO: need UUID
	default:
		// TODO: this error can be better
		return 0, fmt.Errorf("Invalid type %v", f)
	}
}

func (d Table) String() string {
	return d.TableName
}

func removeSpaces(s string) string {
	result := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
	return result
}
