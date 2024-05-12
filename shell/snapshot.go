package shell

import (
	"reflect"

	"github.com/d5/tengo/v2"
)

type (
	snapshot struct {
		items map[string]any

		seen map[tengo.Object]any
	}
)

var (
	snapshoters = map[reflect.Type]func(seen map[tengo.Object]any, v tengo.Object) (any, bool){
		reflect.TypeFor[*tengo.String]():         snapshotAtom,
		reflect.TypeFor[*tengo.Int]():            snapshotAtom,
		reflect.TypeFor[*tengo.Float]():          snapshotAtom,
		reflect.TypeFor[*tengo.Bool]():           snapshotAtom,
		reflect.TypeFor[*tengo.Bytes]():          snapshotAtom,
		reflect.TypeFor[*tengo.Map]():            snapshotMap,
		reflect.TypeFor[*tengo.ImmutableMap]():   snapshotMap,
		reflect.TypeFor[*tengo.Array]():          snapshotArray,
		reflect.TypeFor[*tengo.ImmutableArray](): snapshotArray,
	}
)

func snapshotArray(seen map[tengo.Object]any, v tengo.Object) (any, bool) {
	a := tengo.ToInterface(v).([]any)
	return a, true
}

func snapshotMap(seen map[tengo.Object]any, v tengo.Object) (any, bool) {
	m := tengo.ToInterface(v).(map[string]any)
	return m, true
}

func snapshotAtom(seen map[tengo.Object]any, v tengo.Object) (any, bool) {
	seen[v] = tengo.ToInterface(v)
	return tengo.ToInterface(v), true
}

func (s *snapshot) from(globals map[string]tengo.Object) {
	s.seen = map[tengo.Object]any{}
	s.items = map[string]any{}
	for k, v := range globals {
		out, ok := s.takeOne(v)
		if !ok {
			continue
		}
		println("saving", k)
		s.items[k] = out
	}
}

func (s *snapshot) takeOne(v tengo.Object) (any, bool) {
	if val, found := s.seen[v]; found {
		return val, found
	}

	st := snapshoters[reflect.TypeOf(v)]
	if st == nil {
		// required to avoid loops
		s.seen[v] = nil
		return nil, false
	}
	// required to avoid loops
	s.seen[v] = nil
	val, ok := st(s.seen, v)
	if ok {
		// replace the previous value with the correct one
		s.seen[v] = val
	}
	return val, ok
}
