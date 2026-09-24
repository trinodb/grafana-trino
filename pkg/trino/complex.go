package trino

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
)

type trinoTypeKind int

const (
	kindScalar trinoTypeKind = iota
	kindArray
	kindMap
	kindRow
)

type rowField struct {
	name string
	typ  *trinoType
}

type trinoType struct {
	kind   trinoTypeKind
	elem   *trinoType
	key    *trinoType
	value  *trinoType
	fields []rowField
}

func isComplexType(dbType string) bool {
	upper := strings.ToUpper(dbType)
	for _, prefix := range []string{"ARRAY(", "MAP(", "ROW("} {
		if strings.HasPrefix(upper, prefix) {
			return true
		}
	}
	return false
}

var complexTypeConverter = sqlutil.Converter{
	Name:             "trino complex type to JSON",
	InputScanType:    reflect.TypeOf((*interface{})(nil)).Elem(),
	InputTypeMatcher: isComplexType,
	FrameConverter: sqlutil.FrameConverter{
		FieldType: data.FieldTypeNullableJSON,
		ConvertWithColumn: func(in interface{}, col sql.ColumnType) (interface{}, error) {
			v := *(in.(*interface{}))
			if v == nil {
				return (*json.RawMessage)(nil), nil
			}
			typ := cachedTrinoType(col.DatabaseTypeName())
			var buf bytes.Buffer
			if err := writeJSON(&buf, v, typ); err != nil {
				return nil, err
			}
			msg := json.RawMessage(buf.Bytes())
			return &msg, nil
		},
	},
}

var parsedTypes sync.Map

func cachedTrinoType(dbType string) *trinoType {
	if typ, ok := parsedTypes.Load(dbType); ok {
		return typ.(*trinoType)
	}
	typ, _ := parseTrinoType(dbType)
	parsedTypes.Store(dbType, typ)
	return typ
}

func enableJSONCellInspect(frames data.Frames) {
	for _, frame := range frames {
		for _, field := range frame.Fields {
			if field.Type() != data.FieldTypeNullableJSON {
				continue
			}
			if field.Config == nil {
				field.Config = &data.FieldConfig{}
			}
			if field.Config.Custom == nil {
				field.Config.Custom = map[string]interface{}{}
			}
			if _, ok := field.Config.Custom["inspect"]; !ok {
				field.Config.Custom["inspect"] = true
			}
		}
	}
}

func writeJSON(buf *bytes.Buffer, v interface{}, typ *trinoType) error {
	if v == nil {
		buf.WriteString("null")
		return nil
	}
	if typ == nil {
		typ = &trinoType{kind: kindScalar}
	}
	switch val := v.(type) {
	case []interface{}:
		if typ.kind == kindRow && len(typ.fields) == len(val) && rowFieldsNamed(typ.fields) {
			buf.WriteByte('{')
			for i, f := range typ.fields {
				if i > 0 {
					buf.WriteByte(',')
				}
				if err := writeKey(buf, f.name); err != nil {
					return err
				}
				if err := writeJSON(buf, val[i], f.typ); err != nil {
					return err
				}
			}
			buf.WriteByte('}')
			return nil
		}
		buf.WriteByte('[')
		for i, item := range val {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeJSON(buf, item, elementType(typ, i, len(val))); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
		return nil
	case map[string]interface{}:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var valueType *trinoType
		if typ.kind == kindMap {
			valueType = typ.value
		}
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeKey(buf, k); err != nil {
				return err
			}
			if err := writeJSON(buf, val[k], valueType); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
		return nil
	default:
		b, err := json.Marshal(val)
		if err != nil {
			return fmt.Errorf("cannot encode %v (%T) as JSON: %w", val, val, err)
		}
		buf.Write(b)
		return nil
	}
}

func elementType(typ *trinoType, i, n int) *trinoType {
	switch typ.kind {
	case kindArray:
		return typ.elem
	case kindRow:
		if len(typ.fields) == n {
			return typ.fields[i].typ
		}
	}
	return nil
}

func rowFieldsNamed(fields []rowField) bool {
	seen := make(map[string]bool, len(fields))
	for _, f := range fields {
		if f.name == "" || seen[f.name] {
			return false
		}
		seen[f.name] = true
	}
	return true
}

func writeKey(buf *bytes.Buffer, key string) error {
	b, err := json.Marshal(key)
	if err != nil {
		return err
	}
	buf.Write(b)
	buf.WriteByte(':')
	return nil
}

type typeParser struct {
	s   string
	pos int
}

func parseTrinoType(s string) (*trinoType, error) {
	p := &typeParser{s: s}
	t, err := p.parseType()
	if err != nil {
		return nil, err
	}
	p.skipSpaces()
	if p.pos != len(p.s) {
		return nil, fmt.Errorf("unexpected trailing input in type %q at %d", s, p.pos)
	}
	return t, nil
}

func (p *typeParser) parseType() (*trinoType, error) {
	p.skipSpaces()
	word := strings.ToUpper(p.peekWord())
	switch word {
	case "ARRAY", "MAP", "ROW":
		p.pos += len(word)
		p.skipSpaces()
		if p.peek() == '(' {
			return p.parseComplex(word)
		}
	}
	return p.parseScalar()
}

func (p *typeParser) parseComplex(word string) (*trinoType, error) {
	p.pos++
	var t *trinoType
	switch word {
	case "ARRAY":
		elem, err := p.parseType()
		if err != nil {
			return nil, err
		}
		t = &trinoType{kind: kindArray, elem: elem}
	case "MAP":
		key, err := p.parseType()
		if err != nil {
			return nil, err
		}
		if err := p.expect(','); err != nil {
			return nil, err
		}
		value, err := p.parseType()
		if err != nil {
			return nil, err
		}
		t = &trinoType{kind: kindMap, key: key, value: value}
	case "ROW":
		t = &trinoType{kind: kindRow}
		for {
			f, err := p.parseRowField()
			if err != nil {
				return nil, err
			}
			t.fields = append(t.fields, f)
			p.skipSpaces()
			if p.peek() != ',' {
				break
			}
			p.pos++
		}
	}
	if err := p.expect(')'); err != nil {
		return nil, err
	}
	return t, nil
}

func (p *typeParser) parseRowField() (rowField, error) {
	p.skipSpaces()
	if p.peek() == '"' {
		name, err := p.parseQuoted()
		if err != nil {
			return rowField{}, err
		}
		typ, err := p.parseType()
		return rowField{name: strings.ToLower(name), typ: typ}, err
	}
	start := p.pos
	word := p.peekWord()
	p.pos += len(word)
	p.skipSpaces()
	next := p.peek()
	if word == "" || next == 0 || next == ',' || next == ')' || next == '(' || isMultiWordTypeStart(word, p.peekWord()) {
		p.pos = start
		typ, err := p.parseType()
		return rowField{typ: typ}, err
	}
	typ, err := p.parseType()
	return rowField{name: strings.ToLower(word), typ: typ}, err
}

func isMultiWordTypeStart(first, second string) bool {
	first, second = strings.ToUpper(first), strings.ToUpper(second)
	switch first {
	case "TIME", "TIMESTAMP":
		return second == "WITH" || second == "WITHOUT"
	case "INTERVAL":
		return second == "DAY" || second == "YEAR"
	case "DOUBLE":
		return second == "PRECISION"
	}
	return false
}

func (p *typeParser) parseScalar() (*trinoType, error) {
	start := p.pos
	depth := 0
	for p.pos < len(p.s) {
		switch p.s[p.pos] {
		case '(':
			depth++
		case ')':
			if depth == 0 {
				return p.scalarFrom(start)
			}
			depth--
		case ',':
			if depth == 0 {
				return p.scalarFrom(start)
			}
		case '"':
			if _, err := p.parseQuoted(); err != nil {
				return nil, err
			}
			continue
		}
		p.pos++
	}
	return p.scalarFrom(start)
}

func (p *typeParser) scalarFrom(start int) (*trinoType, error) {
	if strings.TrimSpace(p.s[start:p.pos]) == "" {
		return nil, fmt.Errorf("missing type in %q at %d", p.s, start)
	}
	return &trinoType{kind: kindScalar}, nil
}

func (p *typeParser) parseQuoted() (string, error) {
	p.pos++
	var sb strings.Builder
	for p.pos < len(p.s) {
		c := p.s[p.pos]
		p.pos++
		if c != '"' {
			sb.WriteByte(c)
			continue
		}
		if p.peek() == '"' {
			sb.WriteByte('"')
			p.pos++
			continue
		}
		return sb.String(), nil
	}
	return "", fmt.Errorf("unterminated quoted identifier in %q", p.s)
}

func (p *typeParser) peekWord() string {
	end := p.pos
	for end < len(p.s) {
		c := p.s[end]
		if c == ' ' || c == '(' || c == ')' || c == ',' || c == '"' {
			break
		}
		end++
	}
	return p.s[p.pos:end]
}

func (p *typeParser) peek() byte {
	if p.pos < len(p.s) {
		return p.s[p.pos]
	}
	return 0
}

func (p *typeParser) skipSpaces() {
	for p.pos < len(p.s) && p.s[p.pos] == ' ' {
		p.pos++
	}
}

func (p *typeParser) expect(c byte) error {
	p.skipSpaces()
	if p.peek() != c {
		return fmt.Errorf("expected %q in type %q at %d", c, p.s, p.pos)
	}
	p.pos++
	return nil
}
