// Copyright (c) 2026 Jeb Bisson. MIT License. See LICENSE file in the project root.

package instrumentyaml

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/jebbisson/spice-synth/voice"
	"gopkg.in/yaml.v3"
)

type Operator struct {
	Attack        uint8
	Decay         uint8
	Sustain       uint8
	Release       uint8
	Level         uint8
	Multiply      uint8
	KeyScaleRate  bool
	KeyScaleLevel uint8
	Tremolo       bool
	Vibrato       bool
	Sustaining    bool
	Waveform      uint8
}

type Variant struct {
	Name       string   `yaml:"name"`
	Op1        Operator `yaml:"op1"`
	Op2        Operator `yaml:"op2"`
	Feedback   uint8    `yaml:"feedback"`
	Connection uint8    `yaml:"connection"`
}

type FileInstrument struct {
	Name        string    `yaml:"name"`
	Group       string    `yaml:"group,omitempty"`
	DefaultNote string    `yaml:"default_note,omitempty"`
	Variants    []Variant `yaml:"variants"`
}

type File struct {
	Instruments []FileInstrument `yaml:"instruments"`
}

var validFileInstrumentKeys = map[string]struct{}{
	"name":         {},
	"group":        {},
	"default_note": {},
	"variants":     {},
}

var validVariantKeys = map[string]struct{}{
	"name":       {},
	"op1":        {},
	"op2":        {},
	"feedback":   {},
	"connection": {},
}

var validOperatorKeys = map[string]struct{}{
	"a":  {},
	"d":  {},
	"s":  {},
	"r":  {},
	"l":  {},
	"m":  {},
	"kr": {},
	"kl": {},
	"t":  {},
	"v":  {},
	"su": {},
	"w":  {},
}

func (o *Operator) UnmarshalYAML(node *yaml.Node) error {
	var m map[string]yaml.Node
	if err := node.Decode(&m); err != nil {
		return err
	}
	*o = Operator{}
	for key, valNode := range m {
		key = strings.ToLower(key)
		if _, ok := validOperatorKeys[key]; !ok {
			return fmt.Errorf("unknown operator key %q", key)
		}
		if valNode.Kind == yaml.ScalarNode && valNode.Tag == "!!bool" {
			var boolVal bool
			if err := valNode.Decode(&boolVal); err != nil {
				return fmt.Errorf("invalid boolean for %q: %w", key, err)
			}
			switch key {
			case "kr":
				o.KeyScaleRate = boolVal
			case "t":
				o.Tremolo = boolVal
			case "v":
				o.Vibrato = boolVal
			case "su":
				o.Sustaining = boolVal
			}
			continue
		}
		var val uint8
		if err := valNode.Decode(&val); err != nil {
			return fmt.Errorf("invalid numeric value for %q: %w", key, err)
		}
		switch key {
		case "a":
			o.Attack = val
		case "d":
			o.Decay = val
		case "s":
			o.Sustain = val
		case "r":
			o.Release = val
		case "l":
			o.Level = val
		case "m":
			o.Multiply = val
		case "kr":
			o.KeyScaleRate = val != 0
		case "kl":
			o.KeyScaleLevel = val
		case "t":
			o.Tremolo = val != 0
		case "v":
			o.Vibrato = val != 0
		case "su":
			o.Sustaining = val != 0
		case "w":
			o.Waveform = val
		}
	}
	return o.Validate()
}

func (o Operator) MarshalYAML() (interface{}, error) {
	if err := o.Validate(); err != nil {
		return nil, err
	}
	node := &yaml.Node{Kind: yaml.MappingNode, Style: yaml.FlowStyle}
	appendScalar := func(key, value, tag string) {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value},
		)
	}
	appendScalar("a", fmt.Sprintf("%d", o.Attack), "!!int")
	appendScalar("d", fmt.Sprintf("%d", o.Decay), "!!int")
	appendScalar("s", fmt.Sprintf("%d", o.Sustain), "!!int")
	appendScalar("r", fmt.Sprintf("%d", o.Release), "!!int")
	appendScalar("l", fmt.Sprintf("%d", o.Level), "!!int")
	appendScalar("m", fmt.Sprintf("%d", o.Multiply), "!!int")
	appendScalar("kr", fmt.Sprintf("%t", o.KeyScaleRate), "!!bool")
	appendScalar("kl", fmt.Sprintf("%d", o.KeyScaleLevel), "!!int")
	appendScalar("t", fmt.Sprintf("%t", o.Tremolo), "!!bool")
	appendScalar("v", fmt.Sprintf("%t", o.Vibrato), "!!bool")
	appendScalar("su", fmt.Sprintf("%t", o.Sustaining), "!!bool")
	appendScalar("w", fmt.Sprintf("%d", o.Waveform), "!!int")
	return node, nil
}

func (o Operator) Validate() error {
	if o.Attack > 15 {
		return fmt.Errorf("attack out of range: %d", o.Attack)
	}
	if o.Decay > 15 {
		return fmt.Errorf("decay out of range: %d", o.Decay)
	}
	if o.Sustain > 15 {
		return fmt.Errorf("sustain out of range: %d", o.Sustain)
	}
	if o.Release > 15 {
		return fmt.Errorf("release out of range: %d", o.Release)
	}
	if o.Level > 63 {
		return fmt.Errorf("level out of range: %d", o.Level)
	}
	if o.Multiply > 15 {
		return fmt.Errorf("multiply out of range: %d", o.Multiply)
	}
	if o.KeyScaleLevel > 3 {
		return fmt.Errorf("key scale level out of range: %d", o.KeyScaleLevel)
	}
	if o.Waveform > 3 {
		return fmt.Errorf("waveform out of range: %d", o.Waveform)
	}
	return nil
}

func (v *Variant) UnmarshalYAML(node *yaml.Node) error {
	type rawVariant Variant
	var m map[string]yaml.Node
	if err := node.Decode(&m); err != nil {
		return err
	}
	for key := range m {
		if _, ok := validVariantKeys[strings.ToLower(key)]; !ok {
			return fmt.Errorf("unknown variant key %q", key)
		}
	}
	var raw rawVariant
	if err := node.Decode(&raw); err != nil {
		return err
	}
	*v = Variant(raw)
	return v.Validate()
}

func (fi *FileInstrument) Validate() error {
	if fi == nil {
		return fmt.Errorf("nil instrument")
	}
	if fi.Name == "" {
		return fmt.Errorf("instrument name cannot be empty")
	}
	if fi.DefaultNote != "" {
		if _, err := voice.ParseNote(fi.DefaultNote); err != nil {
			return fmt.Errorf("invalid default_note %q: %w", fi.DefaultNote, err)
		}
	}
	if len(fi.Variants) == 0 {
		return fmt.Errorf("instrument %q must have at least one variant", fi.Name)
	}
	return nil
}

func (fi *FileInstrument) UnmarshalYAML(node *yaml.Node) error {
	type rawFileInstrument FileInstrument
	var m map[string]yaml.Node
	if err := node.Decode(&m); err != nil {
		return err
	}
	for key := range m {
		if _, ok := validFileInstrumentKeys[strings.ToLower(key)]; !ok {
			return fmt.Errorf("unknown instrument key %q", key)
		}
	}
	var raw rawFileInstrument
	if err := node.Decode(&raw); err != nil {
		return err
	}
	*fi = FileInstrument(raw)
	return fi.Validate()
}

func (v *Variant) Validate() error {
	if v == nil {
		return fmt.Errorf("nil variant")
	}
	if v.Name == "" {
		return fmt.Errorf("variant name cannot be empty")
	}
	if err := v.Op1.Validate(); err != nil {
		return fmt.Errorf("op1: %w", err)
	}
	if err := v.Op2.Validate(); err != nil {
		return fmt.Errorf("op2: %w", err)
	}
	if v.Feedback > 7 {
		return fmt.Errorf("feedback out of range: %d", v.Feedback)
	}
	if v.Connection > 1 {
		return fmt.Errorf("connection out of range: %d", v.Connection)
	}
	return nil
}

func (f *File) Validate() error {
	if f == nil {
		return fmt.Errorf("nil file")
	}
	seen := make(map[string]struct{})
	for i := range f.Instruments {
		inst := &f.Instruments[i]
		if err := inst.Validate(); err != nil {
			return err
		}
		for j := range inst.Variants {
			variant := &inst.Variants[j]
			if err := variant.Validate(); err != nil {
				return fmt.Errorf("%s[%d]: %w", inst.Name, j, err)
			}
			key := strings.ToLower(inst.Name + "." + variant.Name)
			if _, ok := seen[key]; ok {
				return fmt.Errorf("duplicate variant key %q", inst.Name+"."+variant.Name)
			}
			seen[key] = struct{}{}
		}
	}
	return nil
}

func LoadFile(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return LoadBytes(data)
}

func LoadBytes(data []byte) (*File, error) {
	f := &File{}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(f); err != nil {
		return nil, err
	}
	if err := f.Validate(); err != nil {
		return nil, err
	}
	return f, nil
}
