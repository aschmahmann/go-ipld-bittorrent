package ipldbencode

import (
	"fmt"
	"io"
	"reflect"

	"github.com/ipld/go-ipld-prime"

	bencode "github.com/anacrolix/torrent/bencode"
)

// Encode serializes a git node to a raw binary form.
func Encode(n ipld.Node, w io.Writer) error {
	bwriter := bencode.NewEncoder(w)
	val, err := decodeIntoInterface(n)
	if err != nil {
		return err
	}

	return bwriter.Encode(val)
}

func decodeIntoInterface(n ipld.Node) (interface{}, error) {
	switch k := n.Kind(); k {
	case ipld.Kind_List:
		iter := n.ListIterator()
		var l []interface{}
		for !iter.Done() {
			_, v, err := iter.Next()
			if err != nil {
				return nil, err
			}
			val, err := decodeIntoInterface(v)
			if err != nil {
				return nil, err
			}
			l = append(l, val)
		}
		return l, nil
	case ipld.Kind_Map:
		iter := n.MapIterator()
		m := make(map[string]interface{})
		for !iter.Done() {
			k, v, err := iter.Next()
			if err != nil {
				return nil, err
			}
			if k.Kind() != ipld.Kind_String {
				return nil, fmt.Errorf("non-string map keys")
			}
			key, err := k.AsString()
			if err != nil {
				return nil, err
			}
			val, err := decodeIntoInterface(v)
			if err != nil {
				return nil, err
			}

			if _, found := m[key]; found {
				return nil, fmt.Errorf("duplicate map entry: %s", key)
			}

			m[key] = val
		}
		return m, nil
	case ipld.Kind_String:
		return n.AsString()
	case ipld.Kind_Int:
		return n.AsInt()
	default:
		return nil, fmt.Errorf("unsupported IPLD kind %s", k)
	}
}

// Decode reads from a reader to fill a NodeAssembler
func Decode(na ipld.NodeAssembler, r io.Reader) error {
	bd := bencode.NewDecoder(r)
	var v interface {}
	if err := bd.Decode(&v); err != nil {
		return err
	}

	return decodeInterfaceToNodeAssembler(na, v)
}

func decodeInterfaceToNodeAssembler(na ipld.NodeAssembler, v interface{}) error {
	switch val := v.(type) {
	case string:
		return na.AssignString(val)
	case int64:
		return na.AssignInt(val)
	case []interface{}:
		la, err := na.BeginList(int64(len(val)))
		if err != nil {
			return err
		}
		for _, elem := range val {
			if err := decodeInterfaceToNodeAssembler(la.AssembleValue(), elem); err != nil {
				return err
			}
		}
		return la.Finish()
	case map[string]interface{}:
		ma, err := na.BeginMap(int64(len(val)))
		if err != nil {
			return err
		}
		for mk, mv := range val {
			mapValAssembler, err := ma.AssembleEntry(mk)
			if err != nil {
				return err
			}
			if err := decodeInterfaceToNodeAssembler(mapValAssembler, mv); err != nil {
				return err
			}
		}
		return ma.Finish()
	default:
		return fmt.Errorf("unsupported type for %v", reflect.TypeOf(val))
	}
}
