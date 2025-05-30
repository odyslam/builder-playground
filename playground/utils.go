package playground

import (
	"fmt"
	"strconv"
	"strings"
)

type nullableUint64Value struct {
	ptr **uint64
}

func (n nullableUint64Value) String() string {
	if *n.ptr == nil {
		return "nil"
	}
	return fmt.Sprintf("%d", **n.ptr)
}

func (n nullableUint64Value) Set(s string) error {
	if s == "" || s == "nil" {
		*n.ptr = nil
		return nil
	}

	val, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return err
	}
	*n.ptr = &val
	return nil
}

func (n nullableUint64Value) Type() string {
	return "uint64"
}

func (n nullableUint64Value) GetNoOptDefVal() string {
	return "0"
}

type MapStringFlag map[string]string

func (n *MapStringFlag) String() string {
	parts := []string{}
	for k, v := range *n {
		parts = append(parts, k+"="+v)
	}
	return "(" + strings.Join(parts, ",") + ")"
}

func (n *MapStringFlag) Type() string {
	return "map(string, string)"
}

func (n *MapStringFlag) Set(s string) error {
	parts := strings.Split(s, "=")
	if len(parts) != 2 {
		return fmt.Errorf("expected k=v for flag")
	}

	k := parts[0]
	v := parts[1]

	if *n == nil {
		(*n) = map[string]string{}
	}
	(*n)[k] = v
	return nil
}
