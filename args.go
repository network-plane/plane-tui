package tui

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ValueSet provides typed accessors for parsed arguments or flags.
type ValueSet struct {
	values map[string]any
}

// newValueSet constructs a ValueSet from a map.
func newValueSet(m map[string]any) ValueSet {
	return ValueSet{values: m}
}

// Raw returns the raw stored value without conversion.
func (v ValueSet) Raw(name string) (any, bool) {
	res, ok := v.values[name]
	return res, ok
}

// String retrieves a string value.
func (v ValueSet) String(name string) string {
	if val, ok := v.values[name]; ok {
		switch t := val.(type) {
		case string:
			return t
		case json.Number:
			return t.String()
		case fmt.Stringer:
			return t.String()
		default:
			return fmt.Sprint(t)
		}
	}
	return ""
}

// Strings returns a slice of strings for repeatable args/flags.
func (v ValueSet) Strings(name string) []string {
	if val, ok := v.values[name]; ok {
		switch t := val.(type) {
		case []string:
			return append([]string(nil), t...)
		case []any:
			res := make([]string, 0, len(t))
			for _, item := range t {
				res = append(res, fmt.Sprint(item))
			}
			return res
		case string:
			return []string{t}
		default:
			return []string{fmt.Sprint(t)}
		}
	}
	return nil
}

// Bool retrieves a boolean value.
func (v ValueSet) Bool(name string) bool {
	if val, ok := v.values[name]; ok {
		switch t := val.(type) {
		case bool:
			return t
		case string:
			b, _ := strconv.ParseBool(t)
			return b
		case json.Number:
			i, _ := t.Int64()
			return i != 0
		}
	}
	return false
}

// Int retrieves an integer value.
func (v ValueSet) Int(name string) int {
	if val, ok := v.values[name]; ok {
		switch t := val.(type) {
		case int:
			return t
		case int64:
			return int(t)
		case float64:
			return int(t)
		case string:
			i, _ := strconv.Atoi(t)
			return i
		case json.Number:
			i, _ := t.Int64()
			return int(i)
		}
	}
	return 0
}

// Float retrieves a float value.
func (v ValueSet) Float(name string) float64 {
	if val, ok := v.values[name]; ok {
		switch t := val.(type) {
		case float64:
			return t
		case float32:
			return float64(t)
		case string:
			f, _ := strconv.ParseFloat(t, 64)
			return f
		case json.Number:
			f, _ := t.Float64()
			return f
		case int:
			return float64(t)
		case int64:
			return float64(t)
		}
	}
	return 0
}

// Duration retrieves a time.Duration value.
func (v ValueSet) Duration(name string) time.Duration {
	if val, ok := v.values[name]; ok {
		switch t := val.(type) {
		case time.Duration:
			return t
		case string:
			d, _ := time.ParseDuration(t)
			return d
		case int64:
			return time.Duration(t)
		case int:
			return time.Duration(t)
		}
	}
	return 0
}

// DecodeJSON decodes the value into the provided destination.
func (v ValueSet) DecodeJSON(name string, dest any) error {
	if val, ok := v.values[name]; ok {
		switch t := val.(type) {
		case string:
			return json.Unmarshal([]byte(t), dest)
		case []byte:
			return json.Unmarshal(t, dest)
		default:
			data, err := json.Marshal(t)
			if err != nil {
				return err
			}
			return json.Unmarshal(data, dest)
		}
	}
	return fmt.Errorf("value %q not present", name)
}

// ArgsParser parses raw args into typed value sets according to specs.
type ArgsParser struct{}

// NewArgsParser constructs an ArgsParser.
func NewArgsParser() *ArgsParser { return &ArgsParser{} }

// Parse parses raw arguments with provided spec metadata.
func (p *ArgsParser) Parse(raw []string, spec CommandSpec) (ValueSet, ValueSet, error) {
	flagDefs := buildFlagIndex(spec.Flags)
	argValues := map[string]any{}
	flagValues := map[string]any{}

	posIndex := 0
	var repeatableArg *ArgSpec
	if len(spec.Args) > 0 {
		last := spec.Args[len(spec.Args)-1]
		if last.Repeatable {
			repeatableArg = &last
		}
	}

	i := 0
	for i < len(raw) {
		token := raw[i]
		if strings.HasPrefix(token, "--") {
			name := strings.TrimPrefix(token, "--")
			value, consumed, err := consumeFlagValue(name, raw, i, flagDefs)
			if err != nil {
				return ValueSet{}, ValueSet{}, err
			}
			if consumed > 0 {
				i += consumed
				flagValues[name] = value
				continue
			}
			if value != nil {
				flagValues[name] = value
			} else {
				flagValues[name] = true
			}
			i++
			continue
		}
		if strings.HasPrefix(token, "-") && token != "-" {
			alias := strings.TrimPrefix(token, "-")
			name, ok := resolveShorthand(alias, spec.Flags)
			if !ok {
				return ValueSet{}, ValueSet{}, fmt.Errorf("unknown flag: -%s", alias)
			}
			value, consumed, err := consumeFlagValue(name, raw, i, flagDefs)
			if err != nil {
				return ValueSet{}, ValueSet{}, err
			}
			if consumed > 0 {
				i += consumed
				flagValues[name] = value
				continue
			}
			if value != nil {
				flagValues[name] = value
			} else {
				flagValues[name] = true
			}
			i++
			continue
		}

		if posIndex >= len(spec.Args) {
			if repeatableArg != nil && posIndex >= len(spec.Args)-1 {
				repeatValues, _ := argValues[repeatableArg.Name].([]string)
				repeatValues = append(repeatValues, token)
				argValues[repeatableArg.Name] = repeatValues
				i++
				continue
			}
			return ValueSet{}, ValueSet{}, fmt.Errorf("unexpected argument: %s", token)
		}

		arg := spec.Args[posIndex]
		if arg.Repeatable {
			repeatValues, _ := argValues[arg.Name].([]string)
			repeatValues = append(repeatValues, token)
			argValues[arg.Name] = repeatValues
		} else {
			argValues[arg.Name] = token
			posIndex++
		}
		i++
	}

	if err := applyDefaultsAndValidate(argValues, spec.Args); err != nil {
		return ValueSet{}, ValueSet{}, err
	}
	if err := applyDefaultsAndValidate(flagValues, spec.Flags); err != nil {
		return ValueSet{}, ValueSet{}, err
	}

	return newValueSet(argValues), newValueSet(flagValues), nil
}

func buildFlagIndex(flags []FlagSpec) map[string]FlagSpec {
	index := make(map[string]FlagSpec, len(flags))
	for _, flag := range flags {
		index[flag.Name] = flag
	}
	return index
}

func resolveShorthand(alias string, flags []FlagSpec) (string, bool) {
	for _, flag := range flags {
		if flag.Shorthand == alias {
			return flag.Name, true
		}
	}
	return "", false
}

func consumeFlagValue(name string, raw []string, pos int, flags map[string]FlagSpec) (any, int, error) {
	flag, ok := flags[name]
	if !ok {
		return nil, 0, fmt.Errorf("unknown flag: --%s", name)
	}

	if strings.Contains(name, "=") {
		return nil, 0, fmt.Errorf("invalid flag name: %s", name)
	}

	token := raw[pos]
	if strings.Contains(token, "=") {
		parts := strings.SplitN(token, "=", 2)
		value, err := castValue(flag.Type, parts[1], flag.EnumValues)
		if err != nil {
			return nil, 0, err
		}
		return value, 1, nil
	}

	if flag.Type == ArgTypeBool {
		return true, 1, nil
	}

	if pos+1 >= len(raw) {
		return nil, 0, fmt.Errorf("flag --%s requires a value", name)
	}

	value := raw[pos+1]
	casted, err := castValue(flag.Type, value, flag.EnumValues)
	if err != nil {
		return nil, 0, err
	}
	return casted, 2, nil
}

func castValue(kind ArgType, raw string, enum []string) (any, error) {
	switch kind {
	case ArgTypeString, "":
		return raw, nil
	case ArgTypeInt:
		i, err := strconv.Atoi(raw)
		if err != nil {
			return nil, err
		}
		return i, nil
	case ArgTypeFloat:
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, err
		}
		return f, nil
	case ArgTypeBool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, err
		}
		return b, nil
	case ArgTypeDuration:
		d, err := time.ParseDuration(raw)
		if err != nil {
			return nil, err
		}
		return d, nil
	case ArgTypeEnum:
		if len(enum) == 0 {
			return raw, nil
		}
		for _, candidate := range enum {
			if candidate == raw {
				return raw, nil
			}
		}
		return nil, fmt.Errorf("value %q not in enum", raw)
	case ArgTypeJSON:
		if !json.Valid([]byte(raw)) {
			return nil, fmt.Errorf("invalid json for value %q", raw)
		}
		var v any
		if err := json.Unmarshal([]byte(raw), &v); err != nil {
			return nil, err
		}
		return v, nil
	default:
		return raw, nil
	}
}

func applyDefaultsAndValidate(target map[string]any, specs any) error {
	switch list := specs.(type) {
	case []ArgSpec:
		for _, arg := range list {
			if _, ok := target[arg.Name]; !ok {
				if arg.Required && arg.Default == nil && !arg.Repeatable {
					return fmt.Errorf("missing required argument: %s", arg.Name)
				}
				if arg.Default != nil {
					target[arg.Name] = arg.Default
				}
			}
		}
	case []FlagSpec:
		for _, flag := range list {
			if flag.Hidden {
				continue
			}
			if _, ok := target[flag.Name]; !ok {
				if flag.Required && flag.Default == nil && flag.Type != ArgTypeBool {
					return fmt.Errorf("missing required flag: --%s", flag.Name)
				}
				if flag.Default != nil {
					target[flag.Name] = flag.Default
				} else if flag.Type == ArgTypeBool {
					target[flag.Name] = false
				}
			}
		}
	default:
		return errors.New("unsupported spec type")
	}
	return nil
}

// FormatUsage renders a usage string from command spec.
func FormatUsage(spec CommandSpec) string {
	var b strings.Builder
	b.WriteString(spec.Name)
	if len(spec.Aliases) > 0 {
		b.WriteString(" (aka ")
		b.WriteString(strings.Join(spec.Aliases, ", "))
		b.WriteString(")")
	}

	for _, arg := range spec.Args {
		b.WriteString(" ")
		name := strings.ToUpper(arg.Name)
		if arg.Repeatable {
			name += "..."
		}
		if arg.Required {
			b.WriteString(fmt.Sprintf("<%s>", name))
		} else {
			b.WriteString(fmt.Sprintf("[%s]", name))
		}
	}

	if len(spec.Flags) > 0 {
		b.WriteString(" [flags]")
	}

	return b.String()
}
