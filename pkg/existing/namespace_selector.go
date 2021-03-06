package existing

/*
Copyright (C) 2018 Expedia Group.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import (
	"errors"
	"reflect"

	"github.com/Telefonica/kube-graffiti/pkg/graffiti"
	"github.com/Telefonica/kube-graffiti/pkg/log"
)

// objectsNamespaceMatchesProvidedSelector decides whether the object/object's namespace matches the namespace selector provided.
// If the object is a namespace then it uses its own labels, otherwise the namespace is looked up and used.
// Cluster scoped objects can not match a namespace selector.
// Namespaces without labels can match a namespace selector with a negative match expression.
func objectsNamespaceMatchesProvidedSelector(obj map[string]interface{}, selector string, nsc namespaceCache) (bool, error) {
	mylog := log.ComponentLogger(componentName, "objectsNamespaceMatchesProvidedSelector")
	mlog := mylog.With().Str("selector", selector).Logger()
	var labels map[string]string

	meta, ok := obj["metadata"].(map[string]interface{})
	if !ok {
		mlog.Error().Msg("object has no metadata")
		return false, errors.New("the object is missing metadata")
	}

	name, _ := meta["namespace"].(string)
	kind, ok := obj["kind"].(string)
	if !ok {
		return false, errors.New("this object seems to have no kind")
	}
	if len(name) == 0 && kind != "Namespace" {
		// Cluster scoped resources (except namespaces) can not match a namespace selector!
		mlog.Debug().Msg("a cluster scoped object can not match any namespace selector")
		return false, nil
	}

	if kind == "Namespace" {
		// match against our labels...
		mlog.Debug().Msg("object is a namespace using obj metadata labels")
		labels = lookupLabels(meta)
	} else {
		mlog.Debug().Str("namespace", name).Msg("object is not a namespace, looking up namespace labels")
		// lookup namespace from the cache
		ns, err := nsc.LookupNamespace(name)
		if err != nil {
			return false, err
		}
		labels = ns.Labels
	}
	if err := graffiti.ValidateLabelSelector(selector); err != nil {
		return false, errors.New("invalid label selector")
	}
	return graffiti.MatchLabelSelector(selector, labels)
}

// lookupLabels accepts any object but wants a map.  It scans for a string key "labels" and returns its value as
// a map[string]string.
// It uses reflection so that it works with both map[string]interface{} maps parsed from json and map[interface{}]interface{} maps
// parsed from yaml.
func lookupLabels(obj interface{}) map[string]string {
	mylog := log.ComponentLogger(componentName, "lookupLabels")

	if reflect.ValueOf(obj).Kind() == reflect.Map {
		mylog.Debug().Msg("object is a map")
		keys := reflect.ValueOf(obj).MapKeys()
		for _, k := range keys {
			ks, _ := getStringValue(k)
			mylog.Debug().Str("key", ks).Msg("checking if key is labels")
			if ks == "labels" {
				mylog.Debug().Msg("found a 'labels' key...")
				return convertToMapStringString(reflect.ValueOf(obj).MapIndex(k))
			}
		}
	} else {
		mylog.Error().Str("kind", reflect.ValueOf(obj).Kind().String()).Msg("object is not a map")
	}
	return make(map[string]string)
}

// convertToMapStringString takes an reflect.Value object and returns a map[string]string of its contents
// assuming it is a) a map and b) contains string keys and string values.
// all other types/values are simply ignored.
func convertToMapStringString(obj reflect.Value) map[string]string {
	mylog := log.ComponentLogger(componentName, "convertToMapStringString")
	labels := make(map[string]string)

	switch obj.Kind() {
	case reflect.Interface:
		mylog.Debug().Msg("object is an interface")
		return convertToMapStringString(reflect.ValueOf(obj.Interface()))
	case reflect.Map:
		mylog.Debug().Msg("object is a map")
		keys := obj.MapKeys()
		for _, k := range keys {
			if ks, ok := getStringValue(k); ok {
				if vs, ok := getStringValue(obj.MapIndex(k)); ok {
					labels[ks] = vs
				}
			}
		}
	default:
		mylog.Error().Msg("object isn't a map or an interface!")
	}
	return labels
}

// getStringValue looks at a reflect.Value object looking for a string, and interating within an interface...
func getStringValue(object reflect.Value) (string, bool) {
	mylog := log.ComponentLogger(componentName, "getStringValue")
	switch object.Kind() {
	case reflect.Interface:
		mylog.Debug().Msg("found an interface")
		return getStringValue(reflect.ValueOf(object.Interface()))
	case reflect.String:
		mylog.Debug().Str("string", object.String()).Msg("found a string")
		return object.String(), true
	default:
		mylog.Error().Str("kind", object.Kind().String()).Msg("did not find an interface or string")
		return "", false
	}
}
