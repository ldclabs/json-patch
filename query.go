// (c) 2022-2022, LDC Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package jsonpatch

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// GetValueByPath returns the value of a given path in a raw encoded JSON document.
func GetValueByPath(doc []byte, path string) ([]byte, error) {
	return NewNode(doc).GetValue(path, nil)
}

// GetChild returns the child node of a given path in the node.
func (n *Node) GetChild(path string, options *Options) (*Node, error) {
	pd, err := n.intoContainer()
	switch {
	case err != nil:
		return nil, fmt.Errorf("unexpected node %q, %v", n.String(), err)
	case pd == nil:
		return nil, fmt.Errorf("unexpected node %q", n.String())
	}

	if options == nil {
		options = NewOptions()
	}
	con, key := findObject(&pd, path, options)
	if con == nil {
		return nil, fmt.Errorf("unable to get child node by path %q, %v", path, ErrMissing)
	}
	return con.get(key, options)
}

// GetValue returns the child node of a given path in the node.
func (n *Node) GetValue(path string, options *Options) (json.RawMessage, error) {
	cn, err := n.GetChild(path, options)
	if err != nil {
		return nil, err
	}
	return cn.MarshalJSON()
}

// FindChildren returns the children nodes that pass the given test operations in the node.
func (n *Node) FindChildren(tests []*PV, options *Options) (result []*PV, err error) {
	if len(tests) == 0 {
		return
	}

	if options == nil {
		options = NewOptions()
	}

	subpaths, err := toSubpaths(tests[0].Path)
	if err != nil {
		return nil, err
	}

	res, err := findChildNodes(n, NewNode(tests[0].Value), "", subpaths, options)
	if err != nil {
		return nil, err
	}
	for _, test := range tests[1:] {
		subpaths, err := toSubpaths(test.Path)
		if err != nil {
			return nil, err
		}
		rs := make([]*nodePV, 0, len(res))
		v := NewNode(test.Value)
		for _, r := range res {
			if assertObject(r.node, subpaths, v, options) {
				rs = append(rs, r)
			}
		}

		res = rs
		if len(res) == 0 {
			break
		}
	}

	for _, r := range res {
		result = append(result, r.pv)
	}
	return
}

// PV represents a node with a path and a raw encoded JSON value.
type PV struct {
	Path  string          `json:"path"`
	Value json.RawMessage `json:"value"`
}

// PVs represents a list of PV.
type PVs []*PV

type nodePV struct {
	pv   *PV
	node *Node
}

func toSubpaths(s string) ([]string, error) {
	subpaths := strings.Split(s, "/")
	if len(subpaths) < 2 || subpaths[0] != "" {
		return nil, fmt.Errorf("invalid query path %q", s)
	}
	return subpaths[1:], nil
}

func findChildNodes(
	node, value *Node, parentpath string, subpaths []string, options *Options,
) (res []*nodePV, err error) {

	node.intoContainer()
	if node.which == eOther {
		return
	}

	if assertObject(node, subpaths, value, options) {
		res = append(res, &nodePV{&PV{parentpath, *node.raw}, node})
	}

	if node.which == eAry {
		for i, n := range node.ary {
			if n == nil {
				continue
			}
			r, e := findChildNodes(
				n, value, parentpath+"/"+strconv.Itoa(i), subpaths, options)
			if e != nil {
				return nil, e
			}
			if len(r) > 0 {
				res = append(res, r...)
			}
		}
	} else {
		for k, n := range node.doc.obj {
			if n == nil {
				continue
			}
			r, e := findChildNodes(
				n, value, parentpath+"/"+encodePatchKey(k), subpaths, options)
			if e != nil {
				return nil, e
			}
			if len(r) > 0 {
				res = append(res, r...)
			}
		}
	}
	return
}

func assertObject(node *Node, subpaths []string, value *Node, options *Options) bool {
	last := len(subpaths) - 1
	doc, _ := node.intoContainer()
	if doc == nil {
		return false
	}

	for i, part := range subpaths {
		next, ok := doc.get(decodePatchKey(part), options)
		if ok != nil {
			return false
		}
		if i == last {
			if next == nil {
				return value.isNull()
			}
			return next.Equal(value)
		}

		if next == nil {
			return false
		}

		doc, _ = next.intoContainer()
		if doc == nil {
			return false
		}
	}
	return false
}
